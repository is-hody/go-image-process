package vips

// #include "label.h"
import "C"
import "unsafe"

// Align represents VIPS_ALIGN
type Align int

// Direction enum
const (
	AlignLow    Align = C.VIPS_ALIGN_LOW
	AlignCenter Align = C.VIPS_ALIGN_CENTRE
	AlignHigh   Align = C.VIPS_ALIGN_HIGH
)

// DefaultFont is the default font to be used for label texts created by govips
const DefaultFont = "sans 10"

// LabelParams represents a text-based label
type LabelParams struct {
	Text      string
	Font      string
	Width     Scalar
	Height    Scalar
	OffsetX   Scalar
	OffsetY   Scalar
	Opacity   float32
	Color     Color
	Alignment Align
}

type vipsLabelOptions struct {
	Text      *C.char
	Font      *C.char
	Width     C.int
	Height    C.int
	OffsetX   C.int
	OffsetY   C.int
	Alignment C.VipsAlign
	DPI       C.int
	Margin    C.int
	Opacity   C.float
	Color     [3]C.double
}

type Watermark struct {
	Width       int
	Height      int
	Rotate      int
	DPI         int
	Margin      int
	Opacity     float32
	NoReplicate bool
	Text        string
	Font        string
	Background  Color
}

type vipsWatermarkOptions struct {
	Width       C.int
	Height      C.int
	DPI         C.int
	Margin      C.int
	Rotate      C.int
	NoReplicate C.int
	Opacity     C.float
	Text        *C.char
	Font        *C.char
	Background  [3]C.double
}

func vipsWatermark(image *C.VipsImage, w Watermark) (*C.VipsImage, error) {
	var out *C.VipsImage

	// Defaults
	noReplicate := 0
	if w.NoReplicate {
		noReplicate = 1
	}

	text := C.CString(w.Text)
	font := C.CString(w.Font)
	background := [3]C.double{C.double(w.Background.R), C.double(w.Background.G), C.double(w.Background.B)}

	opts := vipsWatermarkOptions{
		Width:       C.int(w.Width),
		Height:      C.int(w.Height),
		DPI:         C.int(w.DPI),
		Margin:      C.int(w.Margin),
		Rotate:      C.int(w.Rotate),
		NoReplicate: C.int(noReplicate),
		Opacity:     C.float(w.Opacity),
		Text:        text,
		Font:        font,
		Background:  background,
	}

	defer C.free(unsafe.Pointer(text))
	defer C.free(unsafe.Pointer(font))

	err := C.vips_watermark(image, &out, (*C.WatermarkOptions)(unsafe.Pointer(&opts)))
	if err != 0 {
		return nil, handleImageError(out)
	}

	return out, nil
}

func labelImage(in *C.VipsImage, params *LabelParams) (*C.VipsImage, error) {
	incOpCounter("label")
	var out *C.VipsImage

	text := C.CString(params.Text)
	defer freeCString(text)

	font := C.CString(params.Font)
	defer freeCString(font)

	// todo: release color?
	color := [3]C.double{C.double(params.Color.R), C.double(params.Color.G), C.double(params.Color.B)}

	w := params.Width.GetRounded(int(in.Xsize))
	h := params.Height.GetRounded(int(in.Ysize))
	offsetX := params.OffsetX.GetRounded(int(in.Xsize))
	offsetY := params.OffsetY.GetRounded(int(in.Ysize))

	opts := vipsLabelOptions{
		Text:      text,
		Font:      font,
		Width:     C.int(w),
		Height:    C.int(h),
		OffsetX:   C.int(offsetX),
		OffsetY:   C.int(offsetY),
		Alignment: C.VipsAlign(params.Alignment),
		Opacity:   C.float(params.Opacity),
		Color:     color,
	}

	// todo: release inline pointer?
	err := C.label(in, &out, (*C.LabelOptions)(unsafe.Pointer(&opts)))
	if err != 0 {
		return nil, handleImageError(out)
	}

	return out, nil
}
