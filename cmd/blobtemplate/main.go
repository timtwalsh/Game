// Command blobtemplate writes blob-47 art for terrains: a labelled
// template for artists to draw over, or flat-colour placeholder sheets the
// game and level maker can use before real art exists. See
// docs/LEVEL_MAKER_SPEC.md, "Autotiling" and build-order step 2.
//
//	blobtemplate -out assets/templates [-scale 4] [-terrain grass]
//	    writes assets/templates/blob47_template.png (blob47_template_x4.png
//	    when scaled; grass_blob47_template.png with -terrain). The layout
//	    is the same for every terrain; -terrain only names the file
//	blobtemplate -terrain grass -placeholder 4a9c3b
//	    writes the grass terrain's sheet (its path in world/terrains.toml)
//	    as flat colour; for an edges = false terrain such as black, fills
//	    just its single tile
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"game/shared/world"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "blobtemplate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("blobtemplate", flag.ContinueOnError)
	terrain := fs.String("terrain", "", "terrain name (required with -placeholder; names the template file otherwise)")
	out := fs.String("out", filepath.Join("assets", "templates"), "directory for the template")
	scale := fs.Int("scale", 1, "template only: enlarge by this whole factor, for a reference copy")
	placeholder := fs.String("placeholder", "", "write a flat-colour placeholder sheet in this colour (rrggbb) instead of a template")
	defsPath := fs.String("defs", filepath.Join("world", "terrains.toml"), "world definitions, for -placeholder")
	root := fs.String("root", ".", "for -placeholder: directory sheet paths in -defs are relative to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scale < 1 || *scale > 16 {
		return errors.New("-scale must be 1..16")
	}

	if *placeholder == "" {
		name := "blob47_template"
		if *terrain != "" {
			name = *terrain + "_" + name
		}
		if *scale > 1 {
			name += fmt.Sprintf("_x%d", *scale)
		}
		path := filepath.Join(*out, name+".png")
		if err := writePNG(path, Scale(Template(), *scale)); err != nil {
			return err
		}
		fmt.Println("wrote", path)
		return nil
	}

	if *terrain == "" {
		return errors.New("-placeholder needs -terrain")
	}
	c, err := ParseColour(*placeholder)
	if err != nil {
		return err
	}
	defs, err := world.LoadDefs(*defsPath)
	if err != nil {
		return err
	}
	t := defs.TerrainByName(*terrain)
	if t == nil {
		return fmt.Errorf("%s has no terrain %q", *defsPath, *terrain)
	}
	var sheet *world.Sheet
	var img image.Image
	if t.Edges {
		sheet = defs.SheetByName(t.Sheet)
		img = Placeholder(c)
	} else {
		var cellIdx int
		sheet, cellIdx = defs.SheetOf(t.TileIndex)
		img = FlatSheet(sheet.Cells, cellIdx, c)
	}
	path := filepath.Join(*root, filepath.FromSlash(sheet.Path))
	if err := writePNG(path, img); err != nil {
		return err
	}
	fmt.Println("wrote", path)
	return nil
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
