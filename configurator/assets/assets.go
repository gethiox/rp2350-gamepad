// Package assets embeds the fonts (SIL Open Font License, see OFL-*.txt).
package assets

import _ "embed"

//go:embed fonts/Dosis-SemiBold.ttf
var DosisSemiBold []byte

//go:embed fonts/Dosis-Bold.ttf
var DosisBold []byte

//go:embed fonts/SpaceMono-Bold.ttf
var SpaceMonoBold []byte
