package main

import (
    "os"
    "text/template"
    "strconv"
    "errors"
    "strings"
    "time"
    "math/rand"

    _ "github.com/mattn/go-sqlite3"
)

//structures used in templates
type Post struct {
    BoardN string
    Id int
    Content string
    Time string
    Parent int
    Identifier string
    File string
    Filename string
    Fileinfo string
    Filemime string
    Imgprev string
    Hash string
    Option string
    Pinned bool
    Locked bool
    Anchored bool
    Replies []int
    OnBoard bool
}

type Thread struct {
    BoardN string
    TId string
    BoardDesc string
    Subject string
    Posts []*Post
    Header []string
    HeaderDescs []string
    OmittedPosts int
    OmittedFiles int
    SThemes []string
}

type Board struct {
    Name string
    Desc string
    Threads []*Thread
    Header []string
    HeaderDescs []string
    SThemes []string
    Captcha_list []string
}

type RSS struct {
    Site_name string
    TLD string
    Board string
    Posts []*Post
}

//getting kind of file 
var Filefuncmap = template.FuncMap {
    "imagecheck": func(filemime string) bool {
        if strings.HasPrefix(filemime, "image") {return true}
        return false
    },
    "avcheck": func(filemime string) bool {
        if strings.HasPrefix(filemime, "audio") {return true}
        if strings.HasPrefix(filemime, "video") {return true}
        return false
    },
    "audiocheck": func(filemime string) bool {
        if strings.HasPrefix(filemime, "audio") {return true}
        return false
    },
    "videocheck": func(filemime string) bool {
        if strings.HasPrefix(filemime, "video") {return true}
        return false
    },
    "captcha" : func() int {
        return rand.Intn(len(Captchas))
    },
}

func Dir_check(path string) {

    if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
        err := os.Mkdir(path, os.ModePerm)
        Err_check(err)
        err = os.Mkdir(path + "Files/", os.ModePerm)
        Err_check(err)
    }
}

func Get_subject(parent, board string) string {
    stmts := Checkout()
    defer Checkin(stmts)

    var subject string

    subject_look_stmt := stmts[subject_look_stmt]
    err := subject_look_stmt.QueryRow(parent, board).Scan(&subject)
    Query_err_check(err)

    return subject
}

func Get_omitted(parent, board string) (int, int) {
    stmts := Checkout()
    defer Checkin(stmts)

    var total_posts int
    var total_files int
    var shown_posts int
    var shown_files int

    total_countstmt := stmts[total_count_stmt]
    err := total_countstmt.QueryRow(board, parent).Scan(&total_posts, &total_files)
    Query_err_check(err)

    shown_countstmt := stmts[shown_count_stmt]
    err = shown_countstmt.QueryRow(board, parent).Scan(&shown_posts, &shown_files)
    Query_err_check(err)

    return (total_posts - shown_posts), (total_files - shown_files)
}

//for board pages
func get_threads(board string) []*Thread {
    stmts := Checkout()
    defer Checkin(stmts)

    parent_coll_stmt := stmts[parent_coll_stmt]
    thread_head_stmt := stmts[thread_head_stmt]

    var board_body []*Thread

    //tables will be called a board 
    parent_rows, err := parent_coll_stmt.Query(board)
    Err_check(err)
    defer parent_rows.Close()

    for parent_rows.Next() {
        var fstpst Post
        var filler int
        var pst_coll []*Post

        err = parent_rows.Scan(&fstpst.Id, &filler)
        Err_check(err)
        err = thread_head_stmt.QueryRow(fstpst.Id, board).Scan(&fstpst.Content, &fstpst.Time, &fstpst.Parent, &fstpst.File,
            &fstpst.Filename, &fstpst.Fileinfo, &fstpst.Filemime, &fstpst.Imgprev, &fstpst.Option, &fstpst.Pinned, &fstpst.Locked, &fstpst.Anchored)
        Query_err_check(err)

        pst_coll = append(pst_coll, &fstpst)
        fstpstid := strconv.Itoa(fstpst.Id)

        rst_psts, err := get_posts(fstpstid, board, thread_body_stmt)
        Err_check(err)

        pst_coll = append(pst_coll, rst_psts...)

        sub := Get_subject(fstpstid, board)
        omitted_posts, omitted_files := Get_omitted(fstpstid, board)

        var thr Thread
        if sub != "" {
            thr = Thread{Posts: pst_coll, Subject: sub, OmittedPosts: omitted_posts, OmittedFiles: omitted_files}
        } else {
            thr = Thread{Posts: pst_coll, OmittedPosts: omitted_posts, OmittedFiles: omitted_files}
        }

        board_body = append(board_body, &thr)
    }

    return board_body
}

//for individual threads
func get_posts(parent string, board string, sql_stmt ReadSQL) ([]*Post, error) {

    stmts := Checkout()
    defer Checkin(stmts)

    update_stmt := stmts[sql_stmt]
    update_rep_stmt := stmts[update_rep_stmt]

    rows, err := update_stmt.Query(parent, board)
    Err_check(err)
    defer rows.Close()

    var thread_body []*Post

    for rows.Next() {
        var pst Post
	pst.BoardN = board
		
        err = rows.Scan(&pst.Id, &pst.Content, &pst.Time, &pst.Parent, &pst.File,
            &pst.Filename, &pst.Fileinfo, &pst.Filemime, &pst.Imgprev, &pst.Option, &pst.Pinned, &pst.Locked, &pst.Anchored)
        Err_check(err)

        rep_rows, err := update_rep_stmt.Query(pst.Id, board)
        Err_check(err)

        for rep_rows.Next() {
            var replier int
            rep_rows.Scan(&replier)
            pst.Replies = append(pst.Replies, replier)
        }

        rep_rows.Close()
        thread_body = append(thread_body, &pst)
    }

    return thread_body, err
}

func get_rss(board, parent string) []*Post {
    if len(board) == 0 {board = "home"}

    stmts := Checkout()
    defer Checkin(stmts)

    rss_coll_stmt := stmts[rss_coll_stmt]

    rows, err := rss_coll_stmt.Query(board, parent)
    Err_check(err)
    defer rows.Close()

    var rss_body []*Post

    for rows.Next() {
        var pst Post
		
        err = rows.Scan(&pst.Id, &pst.BoardN, &pst.Content, &pst.Parent, &pst.File, &pst.Imgprev)
        Err_check(err)

        rss_body = append(rss_body, &pst)
    }

    return rss_body
}

func Build_board(board string) {
    boardtemp := template.New("board.html").Funcs(Filefuncmap)
    boardtemp, err := boardtemp.ParseFiles(BP + "/templates/board.html", BP + "/templates/snippet.html")
    Err_check(err)

    path := BP + "head/" + board + "/"
    Dir_check(path)

    f, err := os.Create(path + "index.html")
    Err_check(err)
    defer f.Close()

    threads := get_threads(board)

    cboard := Board{Name: board,  Desc: Board_map[board],Threads: threads,
        Header: Board_names, HeaderDescs: Board_descs, SThemes: Themes, Captcha_list: Captchas}
    boardtemp.Execute(f, cboard)
}

func Build_thread(parent string, board string) { //will accept argument for board and thread number
    posts, err := get_posts(parent, board, update_stmt)
    if len(posts) == 0 {return} 
    sub := Get_subject(parent, board)

    threadtemp := template.New("thread.html").Funcs(Filefuncmap)
    threadtemp, err = threadtemp.ParseFiles(BP + "/templates/thread.html", BP + "/templates/snippet.html")
    Err_check(err)

    path := BP + "head/" + board + "/"
    Dir_check(path)

    f, err := os.Create(path + parent + ".html")
    Err_check(err)
    defer f.Close()

    thr := Thread{BoardN: board, TId: parent, BoardDesc: Board_map[board],
            Posts: posts, Subject: sub,
            Header: Board_names, HeaderDescs: Board_descs, SThemes: Themes}
                
    threadtemp.Execute(f, thr)
}

func Build_rss(board, parent string, newpost ...bool) {
    if len(newpost) > 0 {
        time.Sleep(7 * time.Minute)
    }

    if len(parent) == 0 {
        parent = "rss"
    } else {
        stmts := Checkout()
        defer Checkin(stmts)      

        parent_checkstmt := stmts[parent_check_stmt]
        var parent_result int

        err := parent_checkstmt.QueryRow(parent, board).Scan(&parent_result)
        Query_err_check(err)

        if parent_result == 0 {return}
    }
  

    rsstemp := template.New("rss.xml").Funcs(Filefuncmap)
    rsstemp, err := rsstemp.ParseFiles(BP + "/templates/rss.xml")
    Err_check(err)

    path := BP + "head/" + board
    if len(board) > 0 {path += "/"}
    Dir_check(path)

    f, err := os.Create(path + parent + ".xml")
    Err_check(err)
    defer f.Close()

    posts := get_rss(board, parent)

    crss := RSS{Board: board, TLD: TLD, Site_name: SiteName, Posts: posts}
    rsstemp.Execute(f, crss)
}
