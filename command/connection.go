package main 

import (
    "database/sql"
    sqlite3 "github.com/mattn/go-sqlite3"
)

const Max_conns = 5
var readConns = make(chan map[string]*sql.Stmt, Max_conns)
var writeStrings = make(chan map[string]string, 1)
var writeConn = make(chan *sql.DB, 1) 

//statement strings
const (
    prev_string = `SELECT Content, Time, COALESCE(Filename, '') Filename, COALESCE(Fileinfo, '') Fileinfo, COALESCE(Filemime, '') Filemime,
            COALESCE(Imgprev, '') Imgprev, Option FROM posts WHERE Id = ? AND Board = ?`
    prev_parentstring = `SELECT Parent FROM posts WHERE Id = ? AND Board = ?`
    updatestring = `SELECT Id, Content, Time, Parent, COALESCE(File, '') AS File, COALESCE(Filename, '') AS Filename, 
                COALESCE(Fileinfo, '') AS Fileinfo, COALESCE(Filemime, '') AS Filemime, COALESCE(Imgprev, '') Imgprev, Option, 
                Pinned, Locked, Anchored
                FROM posts WHERE Parent = ? AND Board = ?`
    update_repstring = `SELECT Replier FROM replies WHERE Source = ? AND Board = ?`
    parent_collstring = `WITH temp (TParent, Id) AS (SELECT Parent, MAX(Id) FROM posts WHERE ((instr(Option, 'Sage') = 0 AND Anchored <> 1) OR Id = Parent) AND Board = ?1
            GROUP BY Parent ORDER BY MAX(Id) DESC),
        temp2(Parent, Pinned) AS (SELECT Parent, Pinned FROM posts WHERE Id = Parent AND Board = ?1)
        SELECT Parent, Id FROM temp INNER JOIN temp2 ON temp.TParent = temp2.Parent ORDER BY Pinned DESC, Id DESC LIMIT 15`
    thread_headstring = `SELECT Content, Time, Parent, COALESCE(File, '') AS File, COALESCE(Filename, '') AS Filename, 
                COALESCE(Fileinfo, '') AS Fileinfo, COALESCE(Filemime, '') AS Filemime, COALESCE(Imgprev, '') Imgprev, Option,
                Pinned, Locked, Anchored
                FROM posts WHERE Id = ? AND Board = ?`
    thread_bodystring = `SELECT * FROM (
                SELECT Id, Content, Time, Parent, COALESCE(File, '') AS File, COALESCE(Filename, '') AS Filename, 
                COALESCE(Fileinfo, '') AS Fileinfo, COALESCE(Filemime, '') AS Filemime, COALESCE(Imgprev, '') Imgprev, Option FROM posts 
                WHERE Parent = ? AND Board = ? AND Id != Parent ORDER BY Id DESC LIMIT 5)
                ORDER BY Id ASC`
    thread_collstring = `WITH temp (TParent, Id) AS (SELECT Parent, MAX(Id) FROM posts WHERE ((instr(Option, 'Sage') = 0 AND Anchored <> 1) OR Id = Parent) AND Board = ?1
            GROUP BY Parent ORDER BY MAX(Id) DESC),
        temp2(Parent, Pinned) AS (SELECT Parent, Pinned FROM posts WHERE Id = Parent AND Board = ?1)
        SELECT Parent, Id FROM temp INNER JOIN temp2 ON temp.TParent = temp2.Parent ORDER BY Pinned DESC, Id DESC`
    subject_lookstring = `SELECT Subject FROM subjects WHERE Parent = ? AND Board = ?`
    shown_countstring = `Select COUNT(*), COUNT(File) FROM 
      (SELECT *	FROM posts WHERE Board = ?1 AND Parent = ?2 AND Id <> ?2 ORDER BY Id DESC LIMIT 5)`
    total_countstring = `Select COUNT(*), COUNT(File) FROM posts WHERE Board = ?1 AND Parent = ?2 AND Id <> ?2`
    rss_collstring = `SELECT Id, Board, Content, Parent, COALESCE(File, '') AS File, COALESCE(Imgprev, '') Imgprev
                          FROM posts WHERE (Board = ?1 OR ?1 = "home") AND (Parent = ?2 OR ?2 = "rss")
                          ORDER BY Insertorder DESC LIMIT 20`

    //all inserts(and necessary queries) are preformed in one transaction 
    newpost_wfstring = `INSERT INTO posts(Board, Id, Content, Time, Parent, Identifier, File, Filename, Fileinfo, Filemime, Imgprev, Hash,
        Option, Calendar, Clock, Password, Pinned, Locked, Anchored) 
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1), ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, 0, 0, 
		COALESCE((SELECT Anchored FROM posts WHERE Id = ?4), 0))`
    newpost_nfstring = `INSERT INTO posts(Board, Id, Content, Time, Parent, Identifier, Option, Calendar, Clock, Password, Pinned, Locked, Anchored)
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1), ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, 0, 0, 
		COALESCE((SELECT Anchored FROM posts WHERE Id = ?4), 0))`
	user_edit_string = `UPDATE posts SET Content = ? || '<br><br><div class="editmessage">' || ? || '</div>' 
	    WHERE Calendar >= ? AND Password = ? AND Board = ?`	
    dupcheck_string = `SELECT Parent, Id FROM posts WHERE Hash = ? AND Board = ?`
		
    repadd_string = `INSERT INTO replies(Board, Source, Replier, Password) VALUES (?1, ?2, (SELECT Id FROM latest WHERE Board = ?1) - 1, ?3)`
	repupdate_string = `INSERT INTO replies(Board, Source, Replier, Password) VALUES 
	    (?1, ?2, (SELECT Id FROM posts WHERE Password = ?3 AND Board = ?1 LIMIT 1), ?3)`
    subadd_string = `INSERT INTO subjects(Board, Parent, Subject) VALUES (?, ?, ?)`
    hpadd_string = `INSERT INTO homepost(Board, Id, Content, TrunContent, Parent, Password)
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1) - 1, ?2, ?3, ?4, ?5)`
    htadd_string = `INSERT into homethumb(Board, Id, Parent, Imgprev, Password)
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1) - 1, ?2, ?3, ?4)`
	hpupdate_string = `UPDATE homepost SET Content = ?, TrunContent = ? WHERE Password = ? AND Board = ?`
		
    parent_checkstring = `SELECT COUNT(*)
                FROM posts
                WHERE Parent = ? AND Board = ?`
    threadid_string = `SELECT Id FROM latest WHERE Board = ?`

    Add_token_string = `INSERT INTO tokens(Token, Type, Time) VALUES (?, ?, ?)`
    search_token_string = `SELECT Type FROM tokens WHERE Token = ?`
    delete_token_string = `DELETE FROM tokens WHERE Token = ?`
    remove_tokens_string = `DELETE FROM tokens`
    new_user_string = `INSERT INTO credentials(Username, Hash, Type) VALUES (?, ?, ?)`
    remove_user_string = `DELETE FROM credentials WHERE Username = ? AND Type <> 0`
    search_user_string = `SELECT Hash, Type FROM credentials WHERE Username = ?`

    ban_search_string = `SELECT Expiry, Reason FROM banned WHERE Identifier = ? ORDER BY Insertorder ASC`
    ban_remove_string = `DELETE FROM banned WHERE Identifier = ? AND Expiry = ?`

    get_files_string = `SELECT COALESCE(File, '') AS File, COALESCE(Imgprev, '') AS Imgprev FROM posts WHERE (Id = ?1 OR Parent = ?1) AND Board = ?2`
    get_all_files_string = `SELECT COALESCE(File, '') AS File, Board, COALESCE(Imgprev, '') AS Imgprev FROM posts WHERE (Identifier = (SELECT Identifier FROM posts 
        WHERE Id = ?1 AND Board = ?2))`
    get_all_parents_string = `SELECT Id, Board FROM posts WHERE (Identifier = (SELECT Identifier FROM posts 
        WHERE Id = ?1 AND Board = ?2)) AND Id = Parent`
	user_get_file_string = `SELECT COALESCE(File, '') AS File, COALESCE(Imgprev, '') AS Imgprev FROM posts WHERE Password = ? AND Board = ? LIMIT 1`
		
    delete_post_string = `DELETE FROM posts WHERE (Id = ?1 OR Parent = ?1) AND Board = ?2`
	user_delete_string = `DELETE FROM posts WHERE Calendar >= ?1 AND Password = ?2 AND Board = ?3 AND 
	    ((SELECT COUNT(Id) FROM posts WHERE Parent = (SELECT Id FROM posts WHERE Password = ?2 AND Board = ?3 LIMIT 1) AND Board = ?3) <= 1)`    
		//threads with more than one post cannot be deleted by users
    filedelete_string = `UPDATE posts SET Imgprev = 'deleted', File = 'deleted', Filemime = 'image/webp' WHERE Id = ? and Board = ?`
	user_filedelete_string = `UPDATE posts SET Imgprev = 'deleted', File = 'deleted', Filemime = 'image/webp' WHERE Password = ? and Board = ?`
    delete_all_posts_string = `DELETE FROM posts WHERE (Identifier = (SELECT Identifier FROM posts WHERE Id = ?1 AND Board = ?2))`
    isparent_string = `SELECT IIF(Parent = Id, 1, 0) FROM posts WHERE Id = ? AND Board = ?`
    isparent_string2 = `SELECT IIF(Parent = Id, 1, 0), Id FROM posts WHERE Password = ? AND Board = ?`
    ban_string = `INSERT INTO banned(Identifier, Expiry, Mod, Content, Reason) VALUES ((SELECT Identifier FROM posts WHERE Id = ?1 AND Board = ?2), 
        ?3, ?4, (SELECT Content FROM posts WHERE Id = ?1 AND Board = ?2), ?5)`
    delete_log_string = `INSERT INTO deleted(Identifier, Time, Mod, Content, Reason) VALUES ((SELECT Identifier FROM posts WHERE Id = ?1 AND Board = ?2),
        ?3, ?4, replace(replace((SELECT Content FROM posts WHERE Id = ?1 AND Board = ?2), '<', '&lt;'), '>', '&gt;'), ?5)`
    ban_message_string = `UPDATE posts SET Content = Content || '<br><br><div class="banmessage">(' || ? || ')</div>' WHERE Id = ? AND Board = ?`
	
    get_deleted_string = `SELECT Identifier, Time FROM deleted`
    delete_remove_string = `DELETE FROM deleted WHERE Identifier = ? AND TIME = ?`
    get_expired_tokens_string = `SELECT Token, Time FROM tokens`
    delete_expired_token_string = `DELETE FROM tokens WHERE Token = ? AND TIME = ?`
    get_bans_string = `SELECT Identifier, Expiry FROM banned WHERE Expiry <> '-1'`
    delete_ban_string = `DELETE FROM banned WHERE Identifier || Expiry = ? || ?`

    lock_check_string = `SELECT COALESCE(Locked, 0) AS Locked FROM posts WHERE Parent = ? AND Board = ?`
    lock_string = `UPDATE posts SET Locked = 1 WHERE Id = ? AND Board = ?`
    unlock_string = `UPDATE posts SET Locked = 0 WHERE Id = ? AND Board = ?`
    pin_string = `UPDATE posts SET Pinned = 1 WHERE Id = ? AND Board = ?`
    unpin_string = `UPDATE posts SET Pinned = 0 WHERE Id = ? AND Board = ?`
)

var  WriteStrings = map[string]string{"newpost_wf": newpost_wfstring, "newpost_nf": newpost_nfstring, "user_edit": user_edit_string, "dupcheck": dupcheck_string,
        "repadd": repadd_string, "repupdate": repupdate_string, "subadd": subadd_string, 
		"hpadd": hpadd_string, "htadd": htadd_string, "hpupdate": hpupdate_string,
        "parent_check": parent_checkstring, "threadid" : threadid_string,
        "add_token":  Add_token_string, "search_token": search_token_string, 
        "ban_search": ban_search_string, "ban_remove": ban_remove_string, "delete_token": delete_token_string, "remove_tokens": remove_tokens_string,
        "new_user": new_user_string, "remove_user": remove_user_string,"search_user": search_user_string,
        "get_files": get_files_string, "get_all_files": get_all_files_string, "get_all_parents": get_all_parents_string, "user_get_file": user_get_file_string, 
		"delete_post": delete_post_string, "user_delete": user_delete_string, "filedelete": filedelete_string, "user_filedelete": user_filedelete_string, 
		"delete_all_posts": delete_all_posts_string, "isparent": isparent_string, "isparent2": isparent_string2,
        "ban": ban_string, "delete_log": delete_log_string, 
        "ban_message": ban_message_string, "get_deleted": get_deleted_string, "delete_remove": delete_remove_string,
        "get_expired_tokens": get_expired_tokens_string, "delete_expired_token": delete_expired_token_string,
        "get_bans": get_bans_string, "delete_ban": delete_ban_string,
        "lock_check": lock_check_string, "Lock": lock_string, "Unlock": unlock_string, "Pin": pin_string, "Unpin": unpin_string}

func Checkout() map[string]*sql.Stmt {
        return <-readConns
}
func Checkin(c map[string]*sql.Stmt) {
        readConns <- c
}

func WriteConnCheckout() *sql.DB {
    return <- writeConn
}

func WriteConnCheckin(c *sql.DB) {
    writeConn <- c
}


func Make_Conns() {
    for i := 0; i < Max_conns; i++ {

        //preview statements
        conn1, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        prev_stmt, err := conn1.Prepare(prev_string)
        Err_check(err)

        conn2, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        prev_parentstmt, err := conn2.Prepare(prev_parentstring)
        Err_check(err)


        //thread update statements
        conn3, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        updatestmt, err := conn3.Prepare(updatestring)
        Err_check(err)

        conn4, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        update_repstmt, err := conn4.Prepare(update_repstring)
        Err_check(err)


        //board upate statements
        conn5, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        parent_collstmt, err := conn5.Prepare(parent_collstring)
        Err_check(err)

        conn6, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        thread_headstmt, err := conn6.Prepare(thread_headstring)
        Err_check(err)

        conn7, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        thread_bodystmt, err := conn7.Prepare(thread_bodystring)
        Err_check(err)

        //catalog update statement
        conn10, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)    

        thread_collstmt, err := conn10.Prepare(thread_collstring)
        Err_check(err)
       
        //subject lookup
        conn11, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)    

        subject_lookstmt, err := conn11.Prepare(subject_lookstring)
        Err_check(err)

        conn10a, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        hp_collstmt, err := conn10a.Prepare("SELECT Board, Id, Content, TrunContent, Parent, Password FROM homepost ORDER BY Insertorder DESC")
        Err_check(err)

        conn10b, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        ht_collstmt, err := conn10b.Prepare("SELECT Board, Id, Parent, Imgprev, Password FROM homethumb ORDER BY Insertorder DESC LIMIT 6")
        Err_check(err)

        conn12, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        shown_countstmt, err := conn12.Prepare(shown_countstring)
        Err_check(err)

        conn13, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)

        total_countstmt, err := conn13.Prepare(total_countstring)
        Err_check(err)

        conn14, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)
        
        rss_collstmt, err := conn14.Prepare(rss_collstring)
        Err_check(err)
        
        conn15, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)
        
        parent_checkstmt, err := conn15.Prepare(parent_checkstring)
        Err_check(err)
        
        read_stmts := map[string]*sql.Stmt{"prev": prev_stmt, "prev_parent": prev_parentstmt,
            "update": updatestmt, "update_rep": update_repstmt, "parent_coll": parent_collstmt,
            "thread_head": thread_headstmt, "thread_body": thread_bodystmt,
            "thread_coll": thread_collstmt,"subject_look": subject_lookstmt,
            "hp_coll": hp_collstmt, "ht_coll": ht_collstmt,
            "shown_count": shown_countstmt, "total_count": total_countstmt, "rss_coll": rss_collstmt,
            "parent_check": parent_checkstmt}

        readConns <- read_stmts
    }

    sql.Register("sqlite3wregex",
        &sqlite3.SQLiteDriver{
            Extensions: []string{
                BP + `icu_replace`,
            },
        })
  
    new_conn, err := sql.Open("sqlite3wregex", DB_uri)
    Err_check(err)
    writeConn <- new_conn
}
