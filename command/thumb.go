package main 

import (
    "bytes"
    "image"
    "encoding/xml"
    "io"
    "strings"
    _ "golang.org/x/image/webp"
    
    gih "github.com/corona10/goimagehash"
    "github.com/h2non/bimg"
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

    var selected bimg.Options
    selected.Type = bimg.WEBP

    file_dim, err := bimg.Size(file_buffer) 
    if err != nil {return 0, 0, "", err}

    if file_dim.Height >= file_dim.Width && file_dim.Height > max_dim {
        selected.Height = max_dim
    } else if file_dim.Width > file_dim.Height && file_dim.Width > max_dim {
        selected.Width = max_dim
    }

    newImage, err := bimg.NewImage(file_buffer).Process(selected)
    Err_check(err)
    
    imageReader := bytes.NewReader(newImage)
    img, _, err := image.Decode(imageReader)
    Err_check(err)
    
    hash, err := gih.PerceptionHash(img)
    Err_check(err)

    bimg.Write(file_path + file_pre + "s.webp", newImage)

    return file_dim.Width, file_dim.Height, hash.ToString(), nil
}
