package world

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"game/shared"
)

func richLevel(t *testing.T, d *Defs) *Level {
	t.Helper()
	l := paint(t, "meadow", Point{-3, 7},
		"gggdd",
		"ggwdd",
		"g.wwb",
	)
	l.Isolated = true
	l.Ground.Flags[l.Index(0, 0)] |= GroundLocked
	l.Upper[1].Tile[l.Index(2, 2)] = 201
	l.Upper[1].Flags[l.Index(2, 2)] = TileFlipH
	l.SetOverride(1, 1, OverrideInteraction, 1)
	l.Objects = []shared.GameObject{{
		ID: 1, Kind: "warp", X: 744, Y: 448, CollisionType: shared.CollisionTypePassthrough,
		Properties: map[string]string{"destination": "house", "dest_x": "64.0"},
	}, {ID: 2, Kind: "spawn", X: 8, Y: 8}}
	SolveAll(l, LevelSource{l}, d)
	Compile(l, d)
	return l
}

func TestSaveLoadRoundTrip(t *testing.T) {
	d := testDefs(t)
	dir := t.TempDir()
	l := richLevel(t, d)
	if err := SaveLevel(dir, l); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLevel(filepath.Join(dir, "meadow"+LevelSuffix))
	if err != nil {
		t.Fatal(err)
	}
	if got.Hash == "" || got.Hash != l.Hash {
		t.Errorf("hash after load %q, after save %q", got.Hash, l.Hash)
	}
	if !reflect.DeepEqual(got, l) {
		t.Errorf("round trip differs:\n got %+v\nwant %+v", got, l)
	}
	// No temp files left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("dir holds %d files, want exactly the 2 level files", len(entries))
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	d := testDefs(t)
	a1, b1, err := EncodeLevel(richLevel(t, d))
	if err != nil {
		t.Fatal(err)
	}
	a2, b2, _ := EncodeLevel(richLevel(t, d))
	if !bytes.Equal(a1, a2) || !bytes.Equal(b1, b2) {
		t.Error("encoding the same level twice gave different bytes")
	}
	if versionHash(a1, b1) != versionHash(a2, b2) {
		t.Error("hash is not stable")
	}
	l := richLevel(t, d)
	l.SetTerrain(4, 0, tGrass)
	a3, b3, _ := EncodeLevel(l)
	if versionHash(a3, b3) == versionHash(a1, b1) {
		t.Error("changing a cell did not change the hash")
	}
}

// rawGrid builds an uncompressed grid stream with a custom header, then
// gzips it.
func rawGrid(t *testing.T, magic string, version uint16, w, h uint32, layers uint8, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	hdr := gridHeader{Version: version, Width: w, Height: h, LayerCount: layers}
	copy(hdr.Magic[:], magic)
	binary.Write(zw, binary.LittleEndian, hdr)
	zw.Write(body)
	zw.Close()
	return buf.Bytes()
}

func TestDecodeGridErrors(t *testing.T) {
	d := testDefs(t)
	l := richLevel(t, d)
	tb, gb, err := EncodeLevel(l)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeLevel(tb, gb); err != nil {
		t.Fatalf("valid level failed to decode: %v", err)
	}
	cases := []struct {
		name string
		grid []byte
		want string
	}{
		{"bad magic", rawGrid(t, "NOPE", 1, 5, 3, 4, nil), "bad magic"},
		{"bad version", rawGrid(t, gridMagic, 9, 5, 3, 4, nil), "version 9"},
		{"size mismatch", rawGrid(t, gridMagic, 1, 6, 3, 4, nil), "does not match"},
		{"layer mismatch", rawGrid(t, gridMagic, 1, 5, 3, 3, nil), "3 layers"},
		{"truncated", rawGrid(t, gridMagic, 1, 5, 3, 4, make([]byte, 10)), "truncated"},
		{"trailing", rawGrid(t, gridMagic, 1, 5, 3, 4, make([]byte, 4096)), "trailing"},
		{"not gzip", []byte("hello"), "grid:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := DecodeLevel(tb, c.grid)
			wantErr(t, err, c.want)
		})
	}
}

func TestDecodeTOMLErrors(t *testing.T) {
	d := testDefs(t)
	tb, gb, _ := EncodeLevel(richLevel(t, d))
	cases := []struct{ name, from, to, want string }{
		{"format", "format = 1", "format = 2", "format 2"},
		{"bad name", `name = "meadow"`, `name = "Big Meadow"`, "level name"},
		{"no ground first", `name = "ground"`, `name = "floor"`, "first layer must be ground"},
		{"size", "size = [5, 3]", "size = [0, 3]", "must be positive"},
		{"out of world", "pos = [-3, 7]", "pos = [-16001, 7]", "within"},
		{"unknown key", "isolated = true", "isolated = true\nflavour = 1", "unknown key"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := string(tb)
			if !strings.Contains(src, c.from) {
				t.Fatalf("encoded TOML lacks %q:\n%s", c.from, src)
			}
			_, err := DecodeLevel([]byte(strings.Replace(src, c.from, c.to, 1)), gb)
			wantErr(t, err, c.want)
		})
	}
}

func TestLoadLevelChecksFileName(t *testing.T) {
	d := testDefs(t)
	dir := t.TempDir()
	if err := SaveLevel(dir, richLevel(t, d)); err != nil {
		t.Fatal(err)
	}
	tp, gp := LevelPaths(dir, "meadow")
	os.Rename(tp, filepath.Join(dir, "field"+LevelSuffix))
	os.Rename(gp, filepath.Join(dir, "field"+GridSuffix))
	_, err := LoadLevel(filepath.Join(dir, "field"+LevelSuffix))
	wantErr(t, err, "named")
}
