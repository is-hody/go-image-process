#include <stdlib.h>
#include <vips/vips.h>

typedef struct {
  const char *Text;
  const char *Font;
  int Width;
  int Height;
  int OffsetX;
  int OffsetY;
  VipsAlign Align;
  int DPI;
  int Margin;
  float Opacity;
  double Color[3];
} LabelOptions;

typedef struct {
	int    Width;
	int    Height;
	int    DPI;
	int    Margin;
	int    Rotate;
	int    NoReplicate;
	float  Opacity;
	const char *Text;
    const char *Font;
	double Background[3];
} WatermarkOptions;

int vips_watermark(VipsImage *in, VipsImage **out, WatermarkOptions *o);

int label(VipsImage *in, VipsImage **out, LabelOptions *o);

int text(VipsImage **out, const char *text, const char *font, int width,
         int height, VipsAlign align, int dpi);
