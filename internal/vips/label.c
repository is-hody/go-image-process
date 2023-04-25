
#include "label.h"
#include <stdio.h>

int text(VipsImage **out, const char *text, const char *font, int width,
         int height, VipsAlign align, int dpi) {
  return vips_text(out, text, "font", font, "width", width, "height", height,
                   "align", align, "dpi", dpi, NULL);
}

int vips_watermark_replicate (VipsImage *orig, VipsImage *in, VipsImage **out) {
	VipsImage *cache = vips_image_new();

	if (
		vips_replicate(in, &cache,
			1 + orig->Xsize / in->Xsize,
			1 + orig->Ysize / in->Ysize, NULL) ||
		vips_crop(cache, out, 0, 0, orig->Xsize, orig->Ysize, NULL)
	) {
		g_object_unref(cache);
		return 1;
	}

	g_object_unref(cache);
	return 0;
}

int vips_watermark(VipsImage *in, VipsImage **out, WatermarkOptions *o) {
	double ones[4] = { 1, 1, 1 ,1};

	VipsImage *base = vips_image_new();
	VipsImage **t = (VipsImage **) vips_object_local_array(VIPS_OBJECT(base), 11);
	t[0] = in;

	// Make the mask.
	if (
		vips_text(&t[1], o->Text,
			"dpi", o->DPI,
			"width", o->Width,
			"font", o->Font,
			"align", VIPS_ALIGN_CENTRE,
			"rgba",1,
			"justify",1,
			"spacing",1,
			NULL) ||
		vips_rotate(t[1], &t[2], o->Rotate, NULL)||
		vips_linear1(t[2], &t[3], o->Opacity, 0.0, NULL) ||
		vips_cast(t[3], &t[4], VIPS_FORMAT_UCHAR, NULL) ||
		vips_embed(t[4], &t[5], 0, 0, t[4]->Xsize + o->Margin, t[4]->Ysize + o->Margin, NULL)
		) {
		g_object_unref(base);
		return 1;
	}

	// Replicate if necessary
	if (o->NoReplicate != 1) {
		VipsImage *cache = vips_image_new();
		if (vips_watermark_replicate(t[0], t[5], &cache)) {
			g_object_unref(cache);
			g_object_unref(base);
			return 1;
		}
		g_object_unref(t[5]);
		t[5] = cache;
	}

	// Make the constant image to paint the text with.
	if (
		vips_black(&t[6], 1, 1,"bands",4, NULL) ||
		vips_linear(t[6], &t[7], ones, o->Background, 4, NULL) ||
		vips_cast(t[7], &t[8], VIPS_FORMAT_UCHAR, NULL) ||
		vips_copy(t[8], &t[9], "interpretation", t[0]->Type, NULL) ||
		vips_embed(t[9], &t[10], 0, 0, t[0]->Xsize, t[0]->Ysize, "extend", VIPS_EXTEND_COPY, NULL)
		) {
		g_object_unref(base);
		return 1;
	}

	// Blend the mask and text and write to output.
	if (vips_ifthenelse(t[5], t[10], t[0], out, "blend", TRUE, NULL)) {
		g_object_unref(base);
		return 1;
	}

	g_object_unref(base);
	return 0;
}

int label(VipsImage *in, VipsImage **out, LabelOptions *o) {
  double ones[3] = {1, 1, 1};
  VipsImage *base = vips_image_new();
  VipsImage **t = (VipsImage **)vips_object_local_array(VIPS_OBJECT(base), 9);
  if (vips_text(&t[0], o->Text, "font", o->Font, "width", o->Width, "height",
                o->Height, "align", o->Align, NULL) ||
      vips_linear1(t[0], &t[1], o->Opacity, 0.0, NULL) ||
      vips_cast(t[1], &t[2], VIPS_FORMAT_UCHAR, NULL) ||
      vips_embed(t[2], &t[3], o->OffsetX, o->OffsetY, t[2]->Xsize + o->OffsetX,
                 t[2]->Ysize + o->OffsetY, NULL)) {
    g_object_unref(base);
    return 1;
  }
  if (vips_black(&t[4], 1, 1, NULL) ||
      vips_linear(t[4], &t[5], ones, o->Color, 3, NULL) ||
      vips_cast(t[5], &t[6], VIPS_FORMAT_UCHAR, NULL) ||
      vips_copy(t[6], &t[7], "interpretation", in->Type, NULL) ||
      vips_embed(t[7], &t[8], 0, 0, in->Xsize, in->Ysize, "extend",
                 VIPS_EXTEND_COPY, NULL)) {
    g_object_unref(base);
    return 1;
  }
  if (vips_ifthenelse(t[3], t[8], in, out, "blend", TRUE, NULL)) {
    g_object_unref(base);
    return 1;
  }
  g_object_unref(base);
  return 0;
}
