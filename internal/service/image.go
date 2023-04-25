package service

import (
	bytes2 "bytes"
	"context"
	"encoding/base64"
	"fmt"
	errors2 "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	transportHttp "github.com/go-kratos/kratos/v2/transport/http"
	"go-image-process/internal/conf"
	"go-image-process/internal/vips"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/riff"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "gonum.org/v1/plot"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Image struct {
	imageConf *conf.Image
	pool      *sync.Pool
}

func NewImage(bootstrap *conf.Bootstrap) (ImageInterface, func()) {
	if bootstrap.GetVip() != nil {
		vips.Startup(&vips.Config{
			ConcurrencyLevel: int(bootstrap.GetVip().GetConcurrencylevel()),
			MaxCacheMem:      int(bootstrap.GetVip().GetMaxcachemem()),
			MaxCacheSize:     int(bootstrap.GetVip().GetMaxcachesize()),
		})
	} else {
		vips.Startup(&vips.Config{
			ConcurrencyLevel: 4,
			MaxCacheMem:      0,
			MaxCacheSize:     0,
		})
	}
	vips.LoggingSettings(func(messageDomain string, messageLevel vips.LogLevel, message string) {
		log.Info(message)
	}, vips.LogLevelError)
	return &Image{
			imageConf: bootstrap.GetImage(),
			pool: &sync.Pool{
				New: func() interface{} {
					return new(bytes2.Buffer)
				},
			},
		}, func() {
			vips.Shutdown()
		}
}

type PostImageRequest struct {
	ProcessOpt string `json:"x-oss-process"`
}

func (i Image) ImageHandler(ctx context.Context, httpContext transportHttp.Context) (interface{}, error) {
	var req PostImageRequest
	if err := httpContext.BindQuery(&req); err != nil {
		return nil, errors2.BadRequest("PARAM_ERROR", err.Error())
	}
	m := make(map[string][]string)
	var operations []operation
	var formatOperation *operation
	opts := strings.Split(strings.ReplaceAll(req.ProcessOpt, "image/", ""), "/")
	isInfo := false
	for _, s := range opts {
		split := strings.Split(s, ",")
		m[split[0]] = split[1:]
		op := operation{
			operation: split[0],
			opt:       split[1:],
		}
		if op.operation == "info" {
			isInfo = true
		} else if op.operation == "format" {
			formatOperation = &op
		} else {
			operations = append(operations, op)
		}
	}

	buf := i.pool.Get().(*bytes2.Buffer)
	defer func() {
		buf.Reset()
		i.pool.Put(buf)
	}()
	if _, err := io.Copy(buf, httpContext.Request().Body); err != nil {
		log.Context(ctx).Errorf("io copy error: %v", err)
		return nil, err
	}
	importParams := vips.NewImportParams()
	vipImage, err := vips.LoadImageFromBuffer(buf.Bytes(), importParams)
	if err != nil {
		log.Context(ctx).Errorf("vips new image from buf error: %v", err)
		return nil, err
	}
	defer vipImage.Close()

	if isInfo {
		var info = make(map[string]interface{}, 4)
		info["FileSize"] = map[string]interface{}{"value": buf.Len()}
		info["Format"] = map[string]interface{}{"value": vips.ImageTypes[vipImage.Format()]}
		info["ImageHeight"] = map[string]interface{}{"value": vipImage.Height()}
		info["ImageWidth"] = map[string]interface{}{"value": vipImage.Width()}
		if err := httpContext.JSON(http.StatusOK, info); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if len(operations) == 0 && formatOperation == nil {
		return nil, errors2.BadRequest("unknown opt", "unknown opt")
	}

	for _, op := range operations {
		switch op.operation {
		case "resize":
			opt, err := parseResizeOpt(op.opt)
			if err != nil {
				log.Context(ctx).Errorf("parse resize opt error: %v", err)
				return nil, err
			}
			originWidth := float64(vipImage.Width())
			originHeight := float64(vipImage.Height())

			if opt.w > 0 || opt.h > 0 || opt.l > 0 || opt.s > 0 {
				switch opt.m {
				case "pad":
					fillWideAndHigh(originHeight, originWidth, opt)
					opt.w, opt.h = fixedNum(opt.w, opt.h)
					wScale := float64(opt.w) / originWidth
					hScale := float64(opt.h) / originHeight
					scale := math.Min(wScale, hScale)
					if err = vipImage.ResizeWithVScale(scale, -1, vips.KernelLinear); err != nil {
						log.Context(ctx).Errorf("vips resize with v scale error: %v", err)
						return nil, err
					}

					if len(opt.color) > 0 {
						var r, g, b int64
						if _, err := fmt.Sscanf(opt.color, "%02x%02x%02x", &r, &g, &b); err != nil {
							log.Context(ctx).Errorf("fmt sscanf error: %v", err)
							return nil, err
						}
						backgroundColor := &vips.Color{
							R: uint8(r),
							G: uint8(g),
							B: uint8(b),
						}
						if err = vipImage.EmbedBackground(
							int((float64(opt.w)-float64(vipImage.Width()))/2),
							int((float64(opt.h)-float64(vipImage.Height()))/2),
							opt.w,
							opt.h,
							backgroundColor,
						); err != nil {
							log.Context(ctx).Errorf("vips embed background error: %v", err)
							return nil, err
						}
					} else {
						var extend vips.ExtendStrategy
						if !vipImage.HasAlpha() {
							extend = vips.ExtendWhite
						} else {
							extend = vips.ExtendBackground
						}
						if err = vipImage.Embed(
							int((float64(opt.w)-float64(vipImage.Width()))/2),
							int((float64(opt.h)-float64(vipImage.Height()))/2),
							opt.w,
							opt.h,
							extend,
						); err != nil {
							log.Context(ctx).Errorf("vips embed error: %v", err)
							return nil, err
						}
					}
				case "fill":
					fillWideAndHigh(originHeight, originWidth, opt)
					opt.w, opt.h = fixedNum(opt.w, opt.h)
					if limit(opt, vipImage) || opt.limit == 0 {
						if err = vipImage.ThumbnailWithSize(opt.w, opt.h, vips.InterestingCentre, vips.SizeBoth); err != nil {
							log.Context(ctx).Errorf("vips thumbnail with size error: %v", err)
							return nil, err
						}
					}
				case "fixed":
					fillWideAndHigh(originHeight, originWidth, opt)
					if opt.w > 0 && opt.h > 0 {
						if opt.w > 0 && opt.w < vipImage.Width() && opt.h > 0 && opt.h < vipImage.Height() || opt.limit == 0 {
							if err = vipImage.ThumbnailWithSize(opt.w, opt.h, vips.InterestingAll, vips.SizeForce); err != nil {
								log.Context(ctx).Errorf("vips thumbnail with size error: %v", err)
								return nil, err
							}
						}
					} else {
						if opt.w > 0 && opt.w < vipImage.Width() || opt.h > 0 && opt.h < vipImage.Height() || opt.limit == 0 {
							wScale := float64(opt.w) / originWidth
							hScale := float64(opt.h) / originHeight
							wScale, hScale = fixedNum(wScale, hScale)
							scale := math.Min(wScale, hScale)
							if err = vipImage.ResizeWithVScale(scale, -1, vips.KernelLinear); err != nil {
								log.Context(ctx).Errorf("vips resize with v scale error: %v", err)
								return nil, err
							}
						}
					}
				case "mfit":
					opt.l, opt.s = fixedNum(opt.l, opt.s)
					fillWideAndHigh(originHeight, originWidth, opt)
					if (opt.w > 0 || opt.h > 0) && opt.w < vipImage.Width() && opt.h < vipImage.Height() || opt.limit == 0 {
						wScale := float64(opt.w) / originWidth
						hScale := float64(opt.h) / originHeight
						if err = vipImage.Resize(math.Max(wScale, hScale), vips.KernelLinear); err != nil {
							log.Context(ctx).Errorf("vips resize error: %v", err)
							return nil, err
						}
					}
				default:
					fillWideAndHigh(originHeight, originWidth, opt)
					if opt.w > 0 && opt.w < vipImage.Width() || opt.h > 0 && opt.h < vipImage.Height() || opt.limit == 0 {
						if opt.w == 0 {
							opt.w = vipImage.Width()
						}
						if opt.h == 0 {
							opt.h = vipImage.Height()
						}
						wScale := float64(opt.w) / originWidth
						hScale := float64(opt.h) / originHeight
						if err = vipImage.Resize(math.Min(wScale, hScale), vips.KernelLinear); err != nil {
							log.Context(ctx).Errorf("vips resize error: %v", err)
							return nil, err
						}
					}
				}
			} else if opt.p > 0 {
				if err := vipImage.Resize(float64(opt.p)/100, vips.KernelLinear); err != nil {
					log.Context(ctx).Errorf("vips resize error: %v", err)
					return nil, err
				}
			} else {
				return nil, errors2.BadRequest("PARAM_ERROR", "Missing required param")
			}
		case "watermark":
			watermarkOpt, err := parseWatermarkOpt(ctx, op.opt)
			if err != nil {
				return nil, err
			}
			params := vips.Watermark{
				Text:        watermarkOpt.text,
				Opacity:     float32(watermarkOpt.t) / float32(100),
				Width:       100,
				Rotate:      watermarkOpt.rotate - 360,
				DPI:         72,
				Margin:      20,
				Font:        fmt.Sprintf("fangzhengheiti  %d", watermarkOpt.size),
				NoReplicate: watermarkOpt.fill != 1,
			}
			var r, g, b int64
			if _, err := fmt.Sscanf(watermarkOpt.color, "%02x%02x%02x", &r, &g, &b); err != nil {
				log.Context(ctx).Error(err)
				return nil, err
			}
			backgroundColor := vips.Color{
				R: uint8(r),
				G: uint8(g),
				B: uint8(b),
			}
			params.Background = backgroundColor
			if err := vipImage.AddAlpha(); err != nil {
				log.Context(ctx).Error(err)
				return nil, err
			}
			if err := vipImage.WaterMark(&params); err != nil {
				log.Context(ctx).Error(err)
				return nil, err
			}
		case "blur":
			sigma, minAmpl, err := parseBlurOpt(ctx, op.opt)
			if err != nil {
				return nil, err
			}
			if err := vipImage.GaussianBlur(sigma, minAmpl); err != nil {
				log.Context(ctx).Errorf("vips gaussian blur error: %v", err)
				return nil, err
			}
		default:
			return nil, errors2.BadRequest("unknown opt", "unknown opt")
		}
	}

	var targetFormat = vips.ImageTypes[vipImage.Format()]
	if formatOperation != nil {
		targetFormat, err = parseFormatOpt(formatOperation.opt)
		if err != nil {
			log.Context(ctx).Errorf("parse format opt error: %v", err)
			return nil, err
		}
	}
	resBuf, metadata, err := vipEncode(targetFormat, vipImage, i.imageConf.GetQuality())
	if err != nil {
		log.Context(ctx).Errorf("vips encode error: %v", err)
		return nil, err
	}
	return nil, httpContext.Stream(http.StatusOK, GetMimeTypeByVipImageType(metadata.Format), bytes2.NewBuffer(resBuf))
}

type WatermarkOpt struct {
	color  string
	fill   int
	rotate int
	t      int
	text   string
	size   int
}

func parseWatermarkOpt(ctx context.Context, watermarkOpt []string) (*WatermarkOpt, error) {
	var opt = WatermarkOpt{fill: 0, rotate: 0, t: 100, color: "000000", size: 40}
	for _, o := range watermarkOpt {
		if strings.HasPrefix(o, "text_") {
			s := strings.ReplaceAll(o, "text_", "")
			buf, err := base64.RawStdEncoding.DecodeString(s)
			if err != nil {
				log.Context(ctx).Error(err)
				return nil, err
			}
			opt.text = string(buf)
		} else if strings.HasPrefix(o, "fill_") {
			s := strings.ReplaceAll(o, "fill_", "")
			opt.fill, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "size_") {
			s := strings.ReplaceAll(o, "size_", "")
			opt.size, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "rotate_") {
			s := strings.ReplaceAll(o, "rotate_", "")
			opt.rotate, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "t_") {
			s := strings.ReplaceAll(o, "t_", "")
			opt.t, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "color_") {
			opt.color = strings.ReplaceAll(o, "color_", "")
		}
	}
	if len(opt.text) == 0 {
		return nil, errors2.BadRequest("PARAM_ERROR", "Missing required param: text")
	}
	return &opt, nil
}

type ResizeOpt struct {
	w     int
	h     int
	limit int
	m     string
	color string
	l     int
	s     int
	p     int
}

func parseResizeOpt(resizeOpt []string) (*ResizeOpt, error) {
	var opt = ResizeOpt{limit: 1}
	for _, o := range resizeOpt {
		if strings.HasPrefix(o, "w_") {
			s := strings.ReplaceAll(o, "w_", "")
			opt.w, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "h_") {
			s := strings.ReplaceAll(o, "h_", "")
			opt.h, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "limit_") {
			s := strings.ReplaceAll(o, "limit_", "")
			opt.limit, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "m_") {
			opt.m = strings.ReplaceAll(o, "m_", "")
		} else if strings.HasPrefix(o, "color_") {
			opt.color = strings.ReplaceAll(o, "color_", "")
		} else if strings.HasPrefix(o, "l_") {
			s := strings.ReplaceAll(o, "l_", "")
			opt.l, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "s_") {
			s := strings.ReplaceAll(o, "s_", "")
			opt.s, _ = strconv.Atoi(s)
		} else if strings.HasPrefix(o, "p_") {
			s := strings.ReplaceAll(o, "p_", "")
			opt.p, _ = strconv.Atoi(s)
		}
	}
	//如果图片处理URL中同时指定按宽高缩放和等比缩放参数，则只执行指定宽高缩放
	if opt.p > 0 && (opt.w > 0 || opt.h > 0) {
		opt.p = 0
	}
	//如果指定了缩放模式m，且为目标缩放图的宽度w或目标缩放图的高度h指定了值，则目标缩放图的最长边l或目标缩放图的最短边s的取值不会生效
	if (opt.l > 0 || opt.s > 0) && (opt.w > 0 || opt.h > 0) {
		opt.l = 0
		opt.s = 0
	}
	if opt.w == 0 && opt.h == 0 && opt.l == 0 && opt.s == 0 {
		return nil, errors2.BadRequest("InvalidArgument", "Width and Height can not both be 0.")
	}
	return &opt, nil
}

func parseFormatOpt(formatOpt []string) (string, error) {
	targetFormat := formatOpt[0]
	if len(targetFormat) == 0 {
		return "", errors2.BadRequest("PARAM_ERROR", "Missing required param: format")
	}
	return targetFormat, nil
}

func parseBlurOpt(ctx context.Context, opt []string) (sigma float64, minAmpl float64, err error) {
	var radius float64
	for _, o := range opt {
		if strings.HasPrefix(o, "s_") {
			s := strings.ReplaceAll(o, "s_", "")
			sigma, err = strconv.ParseFloat(s, 64)
			if err != nil {
				log.Context(ctx).Error(err)
				return 0, 0, err
			}
		} else if strings.HasPrefix(o, "r_") {
			s := strings.ReplaceAll(o, "r_", "")
			radius, err = strconv.ParseFloat(s, 64)
			if err != nil {
				log.Context(ctx).Error(err)
				return 0, 0, err
			}
		}
	}
	if sigma == 0 || radius == 0 {
		return 0, 0, errors2.BadRequest("PARAM_ERROR", "Missing required param: sigma or radius")
	}
	minAmpl = 1 - (math.Pow(radius/float64(50), 2) / float64(2))
	return
}

func vipEncode(targetFormat string, vipImage *vips.ImageRef, quality int32) ([]byte, *vips.ImageMetadata, error) {
	var buf []byte
	var err error
	var metadata *vips.ImageMetadata
	switch targetFormat {
	case "jpeg":
		buf, metadata, err = vipImage.ExportJpeg(&vips.JpegExportParams{
			Quality:   int(quality),
			Interlace: true,
		})
	case "png":
		buf, metadata, err = vipImage.ExportPng(&vips.PngExportParams{
			Compression: 6,
			Interlace:   false,
			Palette:     false,
			Quality:     int(quality),
		})
	case "webp":
		buf, metadata, err = vipImage.ExportWebp(&vips.WebpExportParams{
			Quality:         int(quality),
			Lossless:        false,
			NearLossless:    false,
			ReductionEffort: 4,
		})
	case "tiff":
		buf, metadata, err = vipImage.ExportTiff(&vips.TiffExportParams{
			Quality:     int(quality),
			Compression: vips.TiffCompressionLzw,
			Predictor:   vips.TiffPredictorHorizontal,
		})
	case "gif":
		buf, metadata, err = vipImage.ExportGIF(&vips.GifExportParams{
			Quality: int(quality),
		})
	default:
		buf, metadata, err = vipImage.ExportNative()
	}
	return buf, metadata, err
}

type operation struct {
	operation string
	opt       []string
}

func GetMimeTypeByVipImageType(code vips.ImageType) string {
	switch code {
	case vips.ImageTypePNG:
		return "image/png"
	case vips.ImageTypeWEBP:
		return "image/webp"
	case vips.ImageTypeTIFF:
		return "image/tiff"
	case vips.ImageTypeGIF:
		return "image/gif"
	case vips.ImageTypeSVG:
		return "image/svg+xml"
	default:
		return "image/jpeg"
	}
}

func limit(opt *ResizeOpt, vipImage *vips.ImageRef) bool {
	return opt.w < vipImage.Width() && opt.w > 0 || opt.h < vipImage.Height() && opt.h > 0
}

func fillWideAndHigh(originHeight float64, originWidth float64, opt *ResizeOpt) {
	useWideAndHigh := opt.w > 0 || opt.h > 0
	if !useWideAndHigh {
		if originHeight < originWidth {
			opt.h = opt.s
			opt.w = opt.l
		} else {
			opt.h = opt.l
			opt.w = opt.s
		}
	}

}

func fixedNum[T int | float64](n1, n2 T) (T, T) {
	if n1 == 0 {
		return n2, n2
	}
	if n2 == 0 {
		return n1, n1
	}
	return n1, n2
}
