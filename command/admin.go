package main

import (
    "net/http"
    "strings"
    "time"
    "strconv"
    "regexp"
    "io/ioutil"
    "database/sql"
    "text/template"
    "context"
    //"fmt"

    "github.com/google/uuid"
)

const (
    base_query_string = `SELECT Insertorder, Board, Id, Content, Time, Parent, Identifier, COALESCE(File, '') AS File, 
            COALESCE(Filename, '') AS Filename,
            COALESCE(Fileinfo, '') AS Fileinfo, COALESCE(Filemime, '') AS Filemime, COALESCE(Imgprev, '') AS Imgprev,
            COALESCE(Hash, '') AS Hash, Option, Pinned, Locked FROM posts 
            WHERE Parent <> 0`
    query_cap = ` ORDER BY Insertorder DESC`

    ban_log_query_string = `SELECT Identifier, Expiry, Mod, IIF(Expiry <> '-1', Content, 'something heinous') as Content, Reason FROM banned`
    delete_log_query_string = `SELECT Identifier, Time, Mod, Content, Reason FROM deleted`
)

var thread_map = map[string]string{"Pin": pin_str, "Unpin": unpin_str, "Lock": lock_str, "Unlock": unlock_str}

type Query_results struct {
    Posts []*Post
    Auth Acc_type
}

type Ban_result struct {
    Identifier string
    Expiry string
    Mod string
    Content string
    Reason string
}

type Delete_result struct {
    Identifier string
    DTime string
    Mod string
    Content string
    Reason string
}

type Log_result struct {
    BRS []*Ban_result
    DRS []*Delete_result
}

func Moderation_actions(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()

    userSession := Logged_in_check(w, req)
    if userSession == nil {return}

    //use maps for these(no duplicates)
    actions := req.FormValue("actions")
    if Entry_check(w, req, "actions", actions) == 0 {return}

    update_posts := false

    //begin transaction
    new_conn := WriteConnCheckout()
    defer WriteConnCheckin(new_conn)
    new_tx, err := new_conn.Begin()
    Err_check(err)
    defer new_tx.Rollback()

    actiontype := req.FormValue("actiontype")
    if Entry_check(w, req, "actiontype", actiontype ) == 0 {return}

    if actiontype == "on_posts" {
        id := req.FormValue("id")
        if Entry_check(w, req, "id", id) == 0 {return}
        board := req.FormValue("board")	
        if Entry_check(w, req, "board", board) == 0 {return}
        parents := req.FormValue("parents")
        if Entry_check(w, req, "parents", parents) == 0 {return}
        reason := req.FormValue("reason")
        hours := req.FormValue("hours")
        days := req.FormValue("days")

        if strings.HasPrefix(actions, "Ban") {     
            if userSession.acc_type == Maid {
                http.Error(w, "Unauthorized.", http.StatusUnauthorized)
                return
            }
        
            duration := 0
            dint, err := strconv.Atoi(days)
            if err == nil {duration += (dint * 24)}
            hint, err := strconv.Atoi(hours)
            if err == nil {duration += hint}
 
            var ban_expiry time.Time
            if duration == 0 {
                ban_expiry = time.Now().In(Nip).Add(time.Hour * 96)
            } else {
                ban_expiry = time.Now().In(Nip).Add(time.Hour * time.Duration(duration))
            }

            if duration >= 0 {
                _, err = new_tx.ExecContext(ctx, ban_str, id, board, ban_expiry.Format(time.RFC1123), userSession.username, reason)
            } else { //permaban
                _, err = new_tx.ExecContext(ctx, ban_str, id, board, -1, userSession.username, reason)
            }
            Err_check(err)

            ban_message := req.FormValue("banmessage")
            if ban_message != "" {
                _, err = new_tx.ExecContext(ctx, ban_message_str, ban_message, id, board)
                Err_check(err)
                update_posts = true
            }
        }

        if strings.HasSuffix(actions, "Delete") {
            _, err = new_tx.ExecContext(ctx, delete_log_str, id, board, time.Now().In(Nip).Format(time.UnixDate), userSession.username, reason)
			Err_check(err)

	    delete_tree(id, board, new_tx, ctx)

            update_posts = true
        }

        if strings.HasSuffix(actions, "Delete All by User") {
            //DO FOR ALL FILES
            file_rows, err := new_tx.QueryContext(ctx, get_all_files_str, id, board)
            Err_check(err)
            defer file_rows.Close()

            for file_rows.Next() {
                var file_name string
                var file_board string
                var imgprev string

                err = file_rows.Scan(&file_name, &file_board, &imgprev)
                Err_check(err)

                if file_name != "" {
                    file_path := BP + "head/" + file_board + "/Files/"
                    Delete_file(file_path, file_name, imgprev)
            }}
            
            thread_rows, err := new_tx.QueryContext(ctx, get_all_parents_str, id, board)
            Err_check(err)
            defer thread_rows.Close()
            
            other_boards := make(map[string]bool)
            for thread_rows.Next() {
                var PID string
                var PBoard string
                
                err = thread_rows.Scan(&PID, &PBoard)
                Err_check(err)
                
                file_path := BP + "head/" + PBoard + "/"
                Delete_file(file_path, PID + ".html", "")
                Delete_file(file_path, PID + ".xml", "")
                
                if PBoard != board {other_boards[PBoard] = true}
            }
            
            for b, _ := range other_boards {
                defer Build_board(b)
                defer Build_catalog(b)
            }

            _, err = new_tx.ExecContext(ctx, delete_log_str, id, board, time.Now().In(Nip).Format(time.UnixDate), userSession.username, "All Removed.")
            Err_check(err)
            _, err = new_tx.ExecContext(ctx, delete_all_posts_str, id, board)
            Err_check(err)

            update_posts = true
        }
        
        if strings.HasSuffix(actions, "Delete File") {
            file_row := new_tx.QueryRowContext(ctx, `SELECT COALESCE(File, '') File, COALESCE(Imgprev, '') Imgprev 
                FROM posts WHERE Id = ? AND Board = ?`, id, board)
            
            var file_name string
            var imgprev string
            file_row.Scan(&file_name, &imgprev)
            
            if file_name != "" {
                file_path := BP + "head/" + board + "/Files/"
                Delete_file(file_path, file_name, imgprev)
                
                _, err = new_tx.ExecContext(ctx, filedelete_str, id, board)
                Err_check(err)
                
                update_posts = true
            }
        }

		//for pinning and locking
        if chain_str, present := thread_map[actions]; present  {
            if userSession.acc_type == Maid {
                http.Error(w, "Unauthorized.", http.StatusUnauthorized)
                return
            }

            _, err = new_tx.ExecContext(ctx, chain_str, parents, board)
            Err_check(err)

            update_posts = true
        }

        err = new_tx.Commit()
        Err_check(err)

        if update_posts {
            go Build_thread(parents, board)
            go Build_board(board)
            go Build_catalog(board)
            go Build_home()
            go Build_rss(board, parents)
            go Build_rss(board, "")
            go Build_rss("", "")
        }

        http.Redirect(w, req, req.Header.Get("Referer"), 302)
    } else if actiontype == "on_site" {
        if userSession.acc_type != Admin {
            http.Error(w, "Unauthorized.", http.StatusUnauthorized)
            return
        }

        if actions == "newuser" {
            usertype := req.FormValue("usertype")
            if Entry_check(w, req, "usertype", usertype) == 0 {return}

            var rusertype Acc_type
            if usertype == "maid" {
                rusertype = Maid
            } else {rusertype = Mod}

            new_token := uuid.NewString()

            _, err = new_tx.ExecContext(ctx, Add_token_str, new_token, rusertype, time.Now().In(Nip).Format(time.UnixDate))
            Err_check(err)

            w.Write([]byte(html_head +  `<title>User Token</title>
                </head><body><center><br>
                    <p>New Token: ` + new_token +`</p>` + html_foot))
        }

        if actions == "removeuser" {
            username := req.FormValue("username")
            if Entry_check(w, req, "username", username) == 0 {return}

            _, err = new_tx.ExecContext(ctx, remove_user_str, username)
            Err_check(err)


            w.Write([]byte(html_head +  `<title>User Token</title>
                </head><body><center><br>
                    <p>User ` + username +  ` removed.</p>` + html_foot))
        }


        if actions == "removetokens" {
            _, err = new_tx.ExecContext(ctx, remove_tokens_str)
            Err_check(err)

            w.Write([]byte(html_head +  `<title>Token Removal</title>
                </head><body><center><br>
                    <p>Done.` + html_foot))
        }
        
        err = new_tx.Commit()
        Err_check(err)
    }

}


func delete_tree(id, board string, new_tx *sql.Tx, ctx context.Context) {
    //DO FOR ALL FILES
    file_rows, err := new_tx.QueryContext(ctx, get_files_str, id, board)
    Err_check(err)
    defer file_rows.Close()
    
    for file_rows.Next() {
        var file_name string
        var imgprev string
        err = file_rows.Scan(&file_name, &imgprev)
        Err_check(err)
        if file_name != "" {
            file_path := BP + "head/" + board + "/Files/"
            Delete_file(file_path, file_name, imgprev)
    }}
    
    var pcheck bool
    pcheck_row := new_tx.QueryRowContext(ctx, isparent_str, id, board)
    pcheck_row.Scan(&pcheck)
    if pcheck {
            file_path := BP + "head/" + board + "/"
            Delete_file(file_path, id + ".html", "")
            Delete_file(file_path, id + ".xml", "")
    }

    _, err = new_tx.ExecContext(ctx, delete_post_str, id, board)
    Err_check(err)
}





func Unban(w http.ResponseWriter, req *http.Request) {
    userSession := Logged_in_check(w, req)
    if userSession == nil {return}

    ctx := req.Context()

    identity := req.FormValue("identifier")
    expiry := req.FormValue("expiry")

    new_conn := WriteConnCheckout()
    defer WriteConnCheckin(new_conn)
    new_tx, err := new_conn.Begin()
    Err_check(err)
    defer new_tx.Rollback()
    
    _, err = new_tx.ExecContext(ctx, ban_remove_str, identity, expiry)
    Err_check(err)

    err = new_tx.Commit()
    Err_check(err)

    http.Redirect(w, req, req.Header.Get("Referer"), 302)
}

//the console
func Load_console(w http.ResponseWriter, req *http.Request) {
    userSession := Logged_in_check(w, req)
    if userSession == nil {return}

    //put this in a function, with the query string being an input. Every query will return an array of posts
    conn, err := sql.Open("sqlite3", DB_uri)
    Err_check(err)
    defer conn.Close()

    query_string := base_query_string

    //time control
    sdate :=  strings.ReplaceAll(req.FormValue("sdate"), "-", "")
    if userSession.acc_type == Maid {
	now := time.Now().In(Nip)
	then := now.Add(time.Duration(-72) * time.Hour)
        sdate = then.Format("20060102")
    }
    
    if sdate != "" {
        _, err := strconv.Atoi(sdate)
        if err != nil {
            http.Error(w, "Invalid start date.", http.StatusBadRequest)
            return
        }

        query_string += " AND Calendar >= " + sdate
    }

    edate :=  strings.ReplaceAll(req.FormValue("edate"), "-", "")
    if edate != "" {
        _, err := strconv.Atoi(edate)
        if err != nil {
            http.Error(w, "Invalid end date.", http.StatusBadRequest)
            return
        }

        query_string += " AND Calendar <= " + edate
    }

    stime :=  strings.ReplaceAll(req.FormValue("stime"), ":", "")
    if stime != "" {
        _, err := strconv.Atoi(stime)
        if err != nil {
            http.Error(w, "Invalid start time.", http.StatusBadRequest)
            return
        }

        query_string += " AND Clock >= " + stime
    }

    etime :=  strings.ReplaceAll(req.FormValue("etime"), ":", "")
    if etime != "" {
        _, err := strconv.Atoi(etime)
        if err != nil {
            http.Error(w, "Invalid end time.", http.StatusBadRequest)
            return
        }

        query_string += " AND Clock <= " + etime
    }

    //location control
    board :=  req.FormValue("board")
    if board != "" {query_string += ` AND Board = "` + board + `"`}

    parent :=  req.FormValue("parent")
    if parent != "" {
        _, err := strconv.Atoi(parent)
        if err != nil {
            http.Error(w, "Invalid parent.", http.StatusBadRequest)
            return
        }

        query_string += " AND Parent = " + parent
    }

    //identifier
    identifier :=  req.FormValue("identifier")
    if identifier != "" {query_string += ` AND Identifier = "` + identifier + `"`}

    query_string += query_cap

    limit := req.FormValue("limit")
    if limit == "" {
        query_string += " LIMIT 10"
    } else {
        intval, err := strconv.Atoi(limit)
        if err != nil {
            http.Error(w, "Invalid limit.", http.StatusBadRequest)
            return
        }
        
        if intval > 0 {query_string += " LIMIT " + limit}
    }

    query_stmt, err := conn.Prepare(query_string)
    Err_check(err)


    rows, err := query_stmt.Query()
    Err_check(err)
    defer rows.Close()

    var most_recent []*Post
    var filler int

    for rows.Next() {
        var pst Post
        err = rows.Scan(&filler, &pst.BoardN, &pst.Id, &pst.Content, &pst.Time, &pst.Parent, &pst.Identifier, &pst.File,
                        &pst.Filename, &pst.Fileinfo, &pst.Filemime, &pst.Imgprev, &pst.Hash, &pst.Option, &pst.Pinned, &pst.Locked)
        Err_check(err)
        most_recent = append(most_recent, &pst)
    }

    if err == nil {
        mostrecent_temp := template.New("console.html").Funcs(Filefuncmap)
        mostrecent_temp, err := mostrecent_temp.ParseFiles(BP + "/templates/console.html", BP + "/templates/snippet.html")
        Err_check(err)

        results := Query_results{Posts: most_recent, Auth: userSession.acc_type}
	err = mostrecent_temp.Execute(w, results)
	Err_check(err)
    }
}

var ubl = make(map[string]bool)
var nakedlinkreg = regexp.MustCompile(`(?:http|ftp|https):\/\/([a-z|A-Z|0-9|\-|\.]+)`)
var urlreg = regexp.MustCompile(`\A[a-z|0-9|\-|\.]+`)

func Get_bl() {
    clear(ubl)

    resp,err := http.Get(URL_bl)
    Err_check(err)
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    Err_check(err)
    lines := strings.Split(string(body), "\n")

    for _, line := range lines {
        if urlreg.MatchString(line) {
            ubl[line] = true
    }}
}

func Renew_bl() {
    for range time.Tick(12 * time.Hour) {
        Get_bl()
    }
}

func Auto_delete() {
    wholesale_stmt := `SELECT Id, Board, Content, Parent
        FROM posts AS outer WHERE Id > (SELECT Id FROM latest AS inner WHERE inner.Board = outer.Board LIMIT 1) - 6`
        
    auto_delete_stmt := `SELECT Id, Board, Parent
        FROM posts AS outer WHERE Id > (SELECT Id FROM latest AS inner WHERE inner.Board = outer.Board LIMIT 1) - 6 AND
        REGEX_REPLACE('[^a-zA-Z0-9]', Content, '') LIKE '%' || ? || '%'`
    
    for range time.Tick(5 * time.Minute) {
        func() {
            update_posts := false
            boards_to_update := make(map[string]bool)
            threads_to_update := make(map[[2]string]bool)
            
            ctx := context.Background()
            ctx, cancel := context.WithTimeout(ctx, 5 * time.Minute)           
            
            new_conn := WriteConnCheckout()
            defer WriteConnCheckin(new_conn)
            new_tx, err := new_conn.Begin()
            Err_check(err)
            defer new_tx.Rollback()

            to_check, err := new_tx.Query(wholesale_stmt)
            Err_check(err)
            defer to_check.Close()

            for to_check.Next() {
                var cid string
                var cboard string
                var ccontent string
                var cparent string

                err = to_check.Scan(&cid, &cboard, &ccontent, &cparent)
                Err_check(err)

                matches := nakedlinkreg.FindAllStringSubmatch(ccontent, -1)
                for _, match := range matches {
                    if _, prs := ubl[match[1]]; prs {
                        boards_to_update[cboard] = true
                        threads_to_update[[2]string{cparent, cboard}] = true
                        delete_tree(cid, cboard, new_tx, ctx)
                        update_posts = true
                        break
                }}
            }

            for _, phrase := range Auto_phrases {
                to_delete, err := new_tx.Query(auto_delete_stmt, phrase)
                Err_check(err)
                defer to_delete.Close()

                for to_delete.Next() { 
                    update_posts = true
                    var cid string
                    var cboard string
                    var cparent string
                    
                    err = to_delete.Scan(&cid, &cboard, &cparent)
                    Err_check(err)

                    boards_to_update[cboard] = true
                    threads_to_update[[2]string{cparent, cboard}] = true
                    
                    delete_tree(cid, cboard, new_tx, ctx)
            }}

            err = new_tx.Commit()
            Err_check(err)
            cancel()

            if update_posts {
                for thread, _ := range threads_to_update {
                    go Build_thread(thread[0], thread[1])
                    go Build_rss(thread[1], thread[0])
                }
                for board, _ := range boards_to_update {
                    go Build_board(board)
                    go Build_catalog(board)
                    go Build_rss(board, "")
                }
                go Build_home()
                go Build_rss("", "")
            }
        }()
}}

func Clean(expiry time.Duration, get_string, remove_string, time_format string) {
    for range time.Tick(expiry) {
        func() {
            new_conn := WriteConnCheckout()
            defer WriteConnCheckin(new_conn)
            new_tx, err := new_conn.Begin()
            Err_check(err)
            defer new_tx.Rollback()

            deleted_rows, err := new_tx.Query(get_string)
            Err_check(err)
            defer deleted_rows.Close()

            for deleted_rows.Next() {
                var deleted_identity string
                var deleted_time string
                err = deleted_rows.Scan(&deleted_identity, &deleted_time)
                Err_check(err) 
                deleted_actualt, err := time.Parse(time_format, deleted_time)
                Err_check(err) 

                if deleted_actualt.Add(expiry).Before(time.Now().In(Nip)) {	
                    _, err = new_tx.Exec(remove_string, deleted_identity, deleted_time)
                    Err_check(err)
            }}
            err = new_tx.Commit()
            Err_check(err)
        }()
}}

func DB_optomize() {
    new_conn := WriteConnCheckout()
    defer WriteConnCheckin(new_conn)

    new_conn.Exec("VACUUM")
    new_conn.Exec("PRAGMA optimize")
}

func Load_log(w http.ResponseWriter, req *http.Request) {
    userSession := Logged_in_check(w, req)
    if userSession == nil {return}

    if userSession.acc_type == Maid {
        http.Error(w, "Unauthorized.", http.StatusUnauthorized)
        return
    }

    conn, err := sql.Open("sqlite3", DB_uri)
    Err_check(err)
    defer conn.Close()

    var brs []*Ban_result
    var drs []*Delete_result

    ban_log_query_stmt, err := conn.Prepare(ban_log_query_string)
    Err_check(err)
    delete_log_query_stmt, err := conn.Prepare(delete_log_query_string)
    Err_check(err)

    ban_rows, err := ban_log_query_stmt.Query()
    Err_check(err)

    for ban_rows.Next() {
        var br Ban_result
        err = ban_rows.Scan(&br.Identifier, &br.Expiry, &br.Mod, &br.Content, &br.Reason)
        Err_check(err)
        brs = append(brs, &br)
    }

    delete_rows, err := delete_log_query_stmt.Query()
    Err_check(err)

    for delete_rows.Next() {
        var dr Delete_result
        err = delete_rows.Scan(&dr.Identifier, &dr.DTime, &dr.Mod, &dr.Content, &dr.Reason)
        Err_check(err)
        drs = append(drs, &dr)
    }

    log_temp := template.New("log.html")
    log_temp, err = log_temp.ParseFiles(BP + "/templates/log.html")
    Err_check(err)

    results := Log_result{BRS: brs, DRS: drs}
    err = log_temp.Execute(w, results)
    Err_check(err)
}
