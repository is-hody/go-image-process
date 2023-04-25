## Go-Image-Process

### About

Go-Image-Process is a web server provide image process api compatible with aliyun image process using libvips.

### Install vips

see https://www.libvips.org/install.html

### Build project

```shell
make build
```

### Run project

```shell
./bin/go-image-process -conf=./configs/config.yaml
```

### Usage

```shell
 curl --location 'http://127.0.0.1:8080/image?x-oss-process=image/resize,w_512,h_512/watermark,color_000000,fill_1,rotate_315,t_20,text_MTIzMTIz/blur,r_1,s_50/format,webp' \
  --header 'Content-Type: image/png' \
  --data '@/XXX/XXX/sample.png'
```

more info about 'x-oss-process'
param: https://help.aliyun.com/document_detail/44688.html?spm=a2c4g.144582.0.0.4a481e4fJF8Yec

### Already supported image process

- [X] info
- [x] resize
- [x] watermark
- [x] blur
- [x] format
- [ ] crop
- [ ] quality
- [ ] auto-orient
- [ ] circle
- [ ] indexcrop
- [ ] rounded-corners
- [ ] rotate
- [ ] interlace
- [ ] average-hue
- [ ] bright
- [ ] sharpen
- [ ] contrast

### Credits

Thanks to:

- https://github.com/libvips/libvips
- https://github.com/davidbyttow/govips
- https://github.com/go-kratos/kratos