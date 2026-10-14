package main

import (
	"bytes"
	"encoding/xml"
	"image"
	"io"
	"os"
	"strings"

	gih "github.com/corona10/goimagehash"
	"github.com/davidbyttow/govips/v2/vips"
	_ "github.com/gen2brain/jxl"
)

func Sanitize_SVG(input []byte) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(input))
	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buffer)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.ProcInst:
			continue
		case xml.StartElement:
			tagName := strings.ToLower(t.Name.Local)
			if tagName == "script" || tagName == "foreignobject" || tagName == "iframe" {
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			var cleanAttrs []xml.Attr
			for _, attr := range t.Attr {
				attrName := strings.ToLower(attr.Name.Local)
				attrValue := strings.ToLower(attr.Value)

				if attrName == "xmlns" {
					continue
				}

				if strings.HasPrefix(attrName, "on") {
					continue
				}
				if (attrName == "href" || strings.Contains(attrName, "href")) &&
					strings.HasPrefix(strings.TrimSpace(attrValue), "javascript:") {
					continue
				}

				cleanAttrs = append(cleanAttrs, attr)
			}
			t.Attr = cleanAttrs

			if tagName == "svg" {
				t.Name.Space = "http://www.w3.org/2000/svg"
			}

			token = t
		}

		if err := encoder.EncodeToken(token); err != nil {
			return nil, err
		}
	}

	encoder.Flush()
	return buffer.Bytes(), nil
}

func Make_thumb(file_path, file_pre string, file_buffer []byte, max_dim int) (int, int, string, error) {
	newImage, err := vips.NewImageFromBuffer(file_buffer)
	Err_check(err)

	width := newImage.Width()
	height := newImage.Height()

	selected_width := min(max_dim, width)
	selected_height := min(max_dim, height)

	err = newImage.Thumbnail(selected_width, selected_height, vips.InterestingAll)
	Err_check(err)

	newImagBytes, _, err := newImage.ExportJxl(vips.NewJxlExportParams())
	Err_check(err)

	imageReader := bytes.NewReader(newImagBytes)
	img, _, err := image.Decode(imageReader)
	Err_check(err)

	hash, err := gih.PerceptionHash(img)
	Err_check(err)

	f, err := os.OpenFile(file_path+file_pre+"s.jxl", os.O_WRONLY|os.O_CREATE, 0666)
	Err_check(err)
	defer f.Close()

	_, err = f.Write(newImagBytes)
	Err_check(err)

	return width, height, hash.ToString(), nil
}
