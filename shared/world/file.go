package world

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"game/shared"
)

// FormatVersion is the level TOML's `format` value.
const FormatVersion = 1

// GridVersion is the grid file's version field.
const GridVersion = 1

const gridMagic = "LVLG"

// File name suffixes. A level named "meadow" is meadow.level.toml plus
// meadow.grid.gz in the same directory.
const (
	LevelSuffix = ".level.toml"
	GridSuffix  = ".grid.gz"
)

type tomlLevel struct {
	Format   int          `toml:"format"`
	Name     string       `toml:"name"`
	Pos      []int        `toml:"pos"`
	Size     []int        `toml:"size"`
	Isolated bool         `toml:"isolated"`
	Layers   []tomlLayer  `toml:"layers"`
	Objects  []tomlObject `toml:"objects,omitempty"`
}

type tomlLayer struct {
	Name string `toml:"name"`
	ZMin uint32 `toml:"z_min"`
	ZMax uint32 `toml:"z_max"`
}

type tomlObject struct {
	ID            int64             `toml:"id"`
	Kind          string            `toml:"kind"`
	X             float32           `toml:"x"`
	Y             float32           `toml:"y"`
	Z             uint32            `toml:"z"`
	CollisionType string            `toml:"collision_type"`
	Properties    map[string]string `toml:"properties,omitempty"`
}

var collisionTypeNames = map[shared.CollisionType]string{
	shared.CollisionTypeSolid:       "solid",
	shared.CollisionTypePassthrough: "passthrough",
	shared.CollisionTypePlatform:    "platform",
}

func parseCollisionType(s string) (shared.CollisionType, error) {
	if s == "" {
		return shared.CollisionTypeSolid, nil
	}
	for ct, name := range collisionTypeNames {
		if name == s {
			return ct, nil
		}
	}
	return 0, fmt.Errorf("unknown collision_type %q", s)
}

// LevelPaths returns the two file paths for a level named name in dir.
func LevelPaths(dir, name string) (tomlPath, gridPath string) {
	return filepath.Join(dir, name+LevelSuffix), filepath.Join(dir, name+GridSuffix)
}

// LoadLevel reads a level from its .level.toml path; the grid file is found
// next to it.
func LoadLevel(tomlPath string) (*Level, error) {
	if !strings.HasSuffix(tomlPath, LevelSuffix) {
		return nil, fmt.Errorf("%s: level files end in %s", tomlPath, LevelSuffix)
	}
	gridPath := strings.TrimSuffix(tomlPath, LevelSuffix) + GridSuffix
	tomlBytes, err := os.ReadFile(tomlPath)
	if err != nil {
		return nil, err
	}
	gridBytes, err := os.ReadFile(gridPath)
	if err != nil {
		return nil, err
	}
	l, err := DecodeLevel(tomlBytes, gridBytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tomlPath, err)
	}
	if want := strings.TrimSuffix(filepath.Base(tomlPath), LevelSuffix); l.Name != want {
		return nil, fmt.Errorf("%s: level is named %q but the file says %q", tomlPath, l.Name, want)
	}
	return l, nil
}

// DecodeLevel builds a level from the bytes of its two files. It is what
// LoadLevel uses and what a streaming client will use on downloaded bytes.
func DecodeLevel(tomlBytes, gridGz []byte) (*Level, error) {
	var t tomlLevel
	md, err := toml.Decode(string(tomlBytes), &t)
	if err != nil {
		return nil, err
	}
	if und := md.Undecoded(); len(und) > 0 {
		return nil, fmt.Errorf("unknown key %q", und[0].String())
	}
	if t.Format != FormatVersion {
		return nil, fmt.Errorf("format %d, this build reads %d", t.Format, FormatVersion)
	}
	if len(t.Pos) != 2 || len(t.Size) != 2 {
		return nil, errors.New("pos and size must each be [x, y]")
	}
	l := &Level{
		Name:     t.Name,
		Pos:      Point{t.Pos[0], t.Pos[1]},
		W:        t.Size[0],
		H:        t.Size[1],
		Isolated: t.Isolated,
	}
	for _, ly := range t.Layers {
		l.Layers = append(l.Layers, LayerInfo(ly))
	}
	if err := l.validateShape(); err != nil {
		return nil, err
	}
	for i, o := range t.Objects {
		ct, err := parseCollisionType(o.CollisionType)
		if err != nil {
			return nil, fmt.Errorf("object %d: %w", i, err)
		}
		if o.ID < 0 {
			return nil, fmt.Errorf("object %d: negative id", i)
		}
		l.Objects = append(l.Objects, shared.GameObject{
			ID: uint64(o.ID), Kind: o.Kind, X: o.X, Y: o.Y, Z: o.Z,
			CollisionType: ct, Properties: o.Properties,
		})
	}
	l.allocate()
	if err := l.decodeGrid(gridGz); err != nil {
		return nil, fmt.Errorf("grid: %w", err)
	}
	l.Hash = versionHash(tomlBytes, gridGz)
	return l, nil
}

// EncodeLevel returns the bytes of both files. Encoding is deterministic:
// the same level always gives the same bytes, so its hash is stable.
func EncodeLevel(l *Level) (tomlBytes, gridGz []byte, err error) {
	if err := l.validateShape(); err != nil {
		return nil, nil, err
	}
	t := tomlLevel{
		Format:   FormatVersion,
		Name:     l.Name,
		Pos:      []int{l.Pos.X, l.Pos.Y},
		Size:     []int{l.W, l.H},
		Isolated: l.Isolated,
	}
	for _, ly := range l.Layers {
		t.Layers = append(t.Layers, tomlLayer(ly))
	}
	for _, o := range l.Objects {
		name, ok := collisionTypeNames[o.CollisionType]
		if !ok {
			return nil, nil, fmt.Errorf("object %d: unknown collision type %d", o.ID, o.CollisionType)
		}
		t.Objects = append(t.Objects, tomlObject{
			ID: int64(o.ID), Kind: o.Kind, X: o.X, Y: o.Y, Z: o.Z,
			CollisionType: name, Properties: o.Properties,
		})
	}
	var tb bytes.Buffer
	enc := toml.NewEncoder(&tb)
	enc.Indent = ""
	if err := enc.Encode(t); err != nil {
		return nil, nil, err
	}
	gb, err := l.encodeGrid()
	if err != nil {
		return nil, nil, err
	}
	return tb.Bytes(), gb, nil
}

// SaveLevel writes the level's two files into dir and sets l.Hash. Each file
// is written to a temp file and renamed into place, so a crash never leaves
// half a file. The grid goes first: if only one of the pair gets written,
// the size and layer checks on load catch a mismatch.
func SaveLevel(dir string, l *Level) error {
	tb, gb, err := EncodeLevel(l)
	if err != nil {
		return err
	}
	tomlPath, gridPath := LevelPaths(dir, l.Name)
	if err := writeAtomic(gridPath, gb); err != nil {
		return err
	}
	if err := writeAtomic(tomlPath, tb); err != nil {
		return err
	}
	l.Hash = versionHash(tb, gb)
	return nil
}

func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(0o644) // CreateTemp makes 0600
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

func versionHash(tomlBytes, gridGz []byte) string {
	h := sha256.New()
	h.Write(tomlBytes)
	h.Write(gridGz)
	return hex.EncodeToString(h.Sum(nil))
}

// gridHeader is the fixed start of the uncompressed grid stream.
type gridHeader struct {
	Magic      [4]byte
	Version    uint16
	Width      uint32
	Height     uint32
	LayerCount uint8
}

// gridSections lists every grid in file order (docs/LEVEL_MAKER_SPEC.md,
// "File formats"). Encode and decode both walk it, so they can't disagree.
func (l *Level) gridSections() []any {
	s := []any{l.Ground.Terrain, l.Ground.Tile, l.Ground.Under, l.Ground.Flags}
	for _, u := range l.Upper {
		s = append(s, u.Tile, u.Flags)
	}
	return append(s,
		l.Overrides.Mask, l.Overrides.Blocking, l.Overrides.Surface, l.Overrides.Interaction,
		l.Props.Blocking, l.Props.Surface, l.Props.Interaction,
	)
}

func (l *Level) encodeGrid() ([]byte, error) {
	var buf bytes.Buffer
	// Header ModTime stays zero so the output is deterministic.
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	hdr := gridHeader{
		Version: GridVersion, Width: uint32(l.W), Height: uint32(l.H),
		LayerCount: uint8(len(l.Layers)),
	}
	copy(hdr.Magic[:], gridMagic)
	if err := binary.Write(zw, binary.LittleEndian, hdr); err != nil {
		return nil, err
	}
	for _, s := range l.gridSections() {
		if err := binary.Write(zw, binary.LittleEndian, s); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeGrid fills the level's (already allocated) grids.
func (l *Level) decodeGrid(gz []byte) error {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return err
	}
	defer zr.Close()
	var hdr gridHeader
	if err := binary.Read(zr, binary.LittleEndian, &hdr); err != nil {
		return fmt.Errorf("header: %w", err)
	}
	if string(hdr.Magic[:]) != gridMagic {
		return fmt.Errorf("bad magic %q", hdr.Magic[:])
	}
	if hdr.Version != GridVersion {
		return fmt.Errorf("version %d, this build reads %d", hdr.Version, GridVersion)
	}
	if int(hdr.Width) != l.W || int(hdr.Height) != l.H {
		return fmt.Errorf("size %dx%d does not match the TOML's %dx%d", hdr.Width, hdr.Height, l.W, l.H)
	}
	if int(hdr.LayerCount) != len(l.Layers) {
		return fmt.Errorf("%d layers, the TOML lists %d", hdr.LayerCount, len(l.Layers))
	}
	for _, s := range l.gridSections() {
		if err := binary.Read(zr, binary.LittleEndian, s); err != nil {
			return fmt.Errorf("truncated: %w", err)
		}
	}
	// Reading to EOF is also what makes gzip verify its checksum.
	n, err := io.Copy(io.Discard, zr)
	if err != nil {
		return err
	}
	if n != 0 {
		return fmt.Errorf("%d unexpected trailing bytes", n)
	}
	return nil
}
