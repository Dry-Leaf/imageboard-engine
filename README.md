An image board engine written in golang.

## Advantages

Uses CSS instead of JS for some common features

- Expanding thumbnails

- Linked-to post highlighting 

Takes advantage of Nginx capabilities

- Banners without JS

- Theme picker without JS

Other

- Editing or deleting your last post without JS

- Files keep their original name when downloaded 

- Video embedding without JS(uses ytp-dl)

- Supports using an external url blacklist

- Webp thumbnails 

- Uses Sqlite by default 

- No PHP or Perl

- RSS support

## Compile Instructions
sudo apt install build-essential cmake git libvips-dev libavformat-dev libswresample-dev libavcodec-dev libavutil-dev libavformat-dev libswscale-dev

sudo apt install golang-go/bookworm-backports

`Or compile the latest version of Go`

sudo apt install yt-dlp/bookworm-backports

go mod init modules

go mod tidy 

go build --tags "fts5" -o engine *.go

`Note`

The icu_replace.so file was compiled from this repo: https://github.com/gwenn/sqlite-regex-replace-ext

gcc --shared -fPIC -I sqlite-autoconf-3071100 icu_replace.c -o icu_replace.so

`To use nginx`

wget (current version from http://nginx.org/en/download.html)

tar -xzvf nginx-(current version).tar.gz

git clone https://github.com/vision5/ngx_devel_kit

git clone https://github.com/yaoweibin/ngx_http_substitutions_filter_module

cd nginx-(current version)

sudo ./configure --add-module=../ngx_devel_kit --add-module=../ngx_http_substitutions_filter_module

sudo make & make install

## Post compilate instructions

Rename sample_ogai.ini to ogai.ini

Fill in the values in ogai.ini

Start Ogai

Create an admin account with the token `500` on the `new_account.html` page

## Post Formatting
quote: >example

reply: >>1

cross-board reply: >>/board/1

spoiler: \~\~example\~\~

bold: \*\*example\*\*

italics: \_\_example\_\_

code block: \`\`\`
                example
            \`\`\`

shift jis: \@\@\@
               example
           \@\@\@
