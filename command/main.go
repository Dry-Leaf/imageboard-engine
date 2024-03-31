package main

import (
    "database/sql"
    "log"
    "os"
    "math/rand"
    "time"
    "errors"
    "strings"
    "io/fs"
    "net/http"
    //"fmt"

    _ "github.com/mattn/go-sqlite3"
)

var DB_uri string
var DB_path string 

func Err_check(err error) {
    if err != nil {
        log.Fatal(err)
    }
}

func Query_err_check(err error) {
    if err != nil {

        if err == sql.ErrNoRows {
            // there were no rows, but otherwise no error occurred
        } else {
                log.Fatal(err)
            }

    }
}

func Time_report(entry string) {
    log.Printf(entry)
}

func Delete_file(file_path, file_name, imgprev string) {
    name_arr := []string{file_name}
    if imgprev != "" && !strings.HasSuffix(imgprev, "image.webp") {
        name_arr = append(name_arr, imgprev)
    }

    if len(Purge_pass) > 0 {
        for _, name := range name_arr {
            err := os.Truncate(file_path + name, 0)
            Err_check(err)
    
            url_path := SiteScheme + SiteName + "." + TLD + strings.TrimPrefix(file_path, BP + "head") + name
            purge_req, err := http.NewRequest("GET", url_path, nil)
            Err_check(err)
            purge_req.Header.Set("purge-pass", Purge_pass)

            client := &http.Client{}
            _, err = client.Do(purge_req)
            Err_check(err)

            //fmt.Println(url_path)

            err = os.Remove(file_path + name)
            if !errors.Is(err, fs.ErrNotExist) {Err_check(err)}
    }}
}

func main() {

    Load_conf()

    file, err := os.OpenFile(BP + "error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
    Err_check(err)
    defer file.Close()

    log.SetOutput(file)
    log.SetFlags(log.LstdFlags | log.Lmicroseconds)
    rand.Seed(time.Now().UnixNano())

    DB_path = BP + "command/post-coll.db"
    DB_uri = "file://" + DB_path + "?_foreign_keys=on&cache=private&_synchronous=NORMAL&_journal_mode=WAL"
    
    if _, err = os.Stat(DB_path); err != nil {
        New_db()
        Admin_init()
    }
    LatestSeed()
    Make_Conns()
    go Clean(40 * time.Hour, "get_deleted", "delete_remove", time.UnixDate)
    go Clean(10 * time.Minute, "get_expired_tokens", "delete_expired_token", time.UnixDate)
    go Clean(24 * time.Hour, "get_bans", "delete_ban", time.RFC1123)

    if URL_bl != "" {
        Get_bl()
        go Renew_bl()
    }
    go Auto_delete()

    Build_home()
    Build_rss("", "")
    for board, _ := range Board_map{
        Build_board(board)
        Build_catalog(board)
        Build_rss(board, "")
    }
    Listen()
}
