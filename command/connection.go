package main 

import (
    "database/sql"
    sqlite3 "github.com/mattn/go-sqlite3"
)

const Max_conns = 5
var readConns = make(chan map[string]*sql.Stmt, Max_conns)
var writeConn = make(chan *sql.DB, 1) 

//statement strings
const (
    prev_str = `SELECT Content, Time, COALESCE(Filename, '') Filename, COALESCE(Fileinfo, '') Fileinfo, COALESCE(Filemime, '') Filemime,
            COALESCE(Imgprev, '') Imgprev, Option FROM posts WHERE Id = ? AND Board = ?`
    prev_parent_str = `SELECT Parent FROM posts WHERE Id = ? AND Board = ?`
    update_str = `SELECT Id, Content, Time, Parent, COALESCE(File, '') AS File, COALESCE(Filename, '') AS Filename, 
                COALESCE(Fileinfo, '') AS Fileinfo, COALESCE(Filemime, '') AS Filemime, COALESCE(Imgprev, '') Imgprev, Option, 
                Pinned, Locked, Anchored
                FROM posts WHERE Parent = ? AND Board = ?`
    update_rep_str = `SELECT Replier FROM replies WHERE Source = ? AND Board = ?`
    parent_coll_str = `WITH temp (TParent, Id) AS (SELECT Parent, MAX(Id) FROM posts WHERE ((instr(Option, 'Sage') = 0 AND Anchored <> 1) OR Id = Parent) AND Board = ?1
            GROUP BY Parent ORDER BY MAX(Id) DESC),
        temp2(Parent, Pinned) AS (SELECT Parent, Pinned FROM posts WHERE Id = Parent AND Board = ?1)
        SELECT Parent, Id FROM temp INNER JOIN temp2 ON temp.TParent = temp2.Parent ORDER BY Pinned DESC, Id DESC LIMIT 15`
    thread_head_str = `SELECT Content, Time, Parent, COALESCE(File, '') AS File, COALESCE(Filename, '') AS Filename, 
                COALESCE(Fileinfo, '') AS Fileinfo, COALESCE(Filemime, '') AS Filemime, COALESCE(Imgprev, '') Imgprev, Option,
                Pinned, Locked, Anchored
                FROM posts WHERE Id = ? AND Board = ?`
    thread_body_str = `SELECT * FROM (
                SELECT Id, Content, Time, Parent, COALESCE(File, '') AS File, COALESCE(Filename, '') AS Filename, 
                COALESCE(Fileinfo, '') AS Fileinfo, COALESCE(Filemime, '') AS Filemime, COALESCE(Imgprev, '') Imgprev, Option FROM posts 
                WHERE Parent = ? AND Board = ? AND Id != Parent ORDER BY Id DESC LIMIT 5)
                ORDER BY Id ASC`
    thread_coll_str = `WITH temp (TParent, Id) AS (SELECT Parent, MAX(Id) FROM posts WHERE ((instr(Option, 'Sage') = 0 AND Anchored <> 1) OR Id = Parent) AND Board = ?1
            GROUP BY Parent ORDER BY MAX(Id) DESC),
        temp2(Parent, Pinned) AS (SELECT Parent, Pinned FROM posts WHERE Id = Parent AND Board = ?1)
        SELECT Parent, Id FROM temp INNER JOIN temp2 ON temp.TParent = temp2.Parent ORDER BY Pinned DESC, Id DESC`
    subject_look_str = `SELECT Subject FROM subjects WHERE Parent = ? AND Board = ?`
    shown_count_str = `Select COUNT(*), COUNT(File) FROM 
      (SELECT *	FROM posts WHERE Board = ?1 AND Parent = ?2 AND Id <> ?2 ORDER BY Id DESC LIMIT 5)`
    total_count_str = `Select COUNT(*), COUNT(File) FROM posts WHERE Board = ?1 AND Parent = ?2 AND Id <> ?2`
    rss_coll_str = `SELECT Id, Board, Content, Parent, COALESCE(File, '') AS File, COALESCE(Imgprev, '') Imgprev
                          FROM posts WHERE (Board = ?1 OR ?1 = "home") AND (Parent = ?2 OR ?2 = "rss")
                          ORDER BY Insertorder DESC LIMIT 20`

    //all inserts(and necessary queries) are preformed in one transaction 
    newpost_wf_str = `INSERT INTO posts(Board, Id, Content, Time, Parent, Identifier, File, Filename, Fileinfo, Filemime, Imgprev, Hash,
        Option, Calendar, Clock, Password, Pinned, Locked, Anchored) 
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1), ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, 0, 0, 
		COALESCE((SELECT Anchored FROM posts WHERE Id = ?4), 0))`
    newpost_nf_str = `INSERT INTO posts(Board, Id, Content, Time, Parent, Identifier, Option, Calendar, Clock, Password, Pinned, Locked, Anchored)
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1), ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, 0, 0, 
		COALESCE((SELECT Anchored FROM posts WHERE Id = ?4), 0))`
    user_edit_str = `UPDATE posts SET Content = ? || '<br><br><div class="editmessage">' || ? || '</div>' 
	    WHERE Calendar >= ? AND Password = ? AND Board = ?`	
    dupcheck_str = `SELECT Parent, Id FROM posts WHERE Hash = ? AND Board = ?`
		
    repadd_str = `INSERT INTO replies(Board, Source, Replier, Password) VALUES (?1, ?2, (SELECT Id FROM latest WHERE Board = ?1) - 1, ?3)`
    repupdate_str = `INSERT INTO replies(Board, Source, Replier, Password) VALUES 
	    (?1, ?2, (SELECT Id FROM posts WHERE Password = ?3 AND Board = ?1 LIMIT 1), ?3)`
    subadd_str = `INSERT INTO subjects(Board, Parent, Subject) VALUES (?, ?, ?)`
    hpadd_str = `INSERT INTO homepost(Board, Id, Content, TrunContent, Parent, Password)
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1) - 1, ?2, ?3, ?4, ?5)`
    htadd_str = `INSERT into homethumb(Board, Id, Parent, Imgprev, Password)
        VALUES (?1, (SELECT Id FROM latest WHERE Board = ?1) - 1, ?2, ?3, ?4)`
    hpupdate_str = `UPDATE homepost SET Content = ?, TrunContent = ? WHERE Password = ? AND Board = ?`
		
    parent_check_str = `SELECT COUNT(*)
                FROM posts
                WHERE Parent = ? AND Board = ?`
    threadid_str = `SELECT Id FROM latest WHERE Board = ?`

    Add_token_str = `INSERT INTO tokens(Token, Type, Time) VALUES (?, ?, ?)`
    search_token_str = `SELECT Type FROM tokens WHERE Token = ?`
    delete_token_str = `DELETE FROM tokens WHERE Token = ?`
    remove_tokens_str = `DELETE FROM tokens`
    new_user_str = `INSERT INTO credentials(Username, Hash, Type) VALUES (?, ?, ?)`
    remove_user_str = `DELETE FROM credentials WHERE Username = ? AND Type <> 0`
    search_user_str = `SELECT Hash, Type FROM credentials WHERE Username = ?`

    ban_search_str = `SELECT Expiry, Reason FROM banned WHERE Identifier = ? ORDER BY Insertorder ASC`
    ban_remove_str = `DELETE FROM banned WHERE Identifier = ? AND Expiry = ?`

    get_files_str = `SELECT COALESCE(File, '') AS File, COALESCE(Imgprev, '') AS Imgprev FROM posts WHERE (Id = ?1 OR Parent = ?1) AND Board = ?2`
    get_all_files_str = `SELECT COALESCE(File, '') AS File, Board, COALESCE(Imgprev, '') AS Imgprev FROM posts WHERE (Identifier = (SELECT Identifier FROM posts 
        WHERE Id = ?1 AND Board = ?2))`
    get_all_parents_str = `SELECT Id, Board FROM posts WHERE (Identifier = (SELECT Identifier FROM posts 
        WHERE Id = ?1 AND Board = ?2)) AND Id = Parent`
    user_get_file_str = `SELECT COALESCE(File, '') AS File, COALESCE(Imgprev, '') AS Imgprev FROM posts WHERE Password = ? AND Board = ? LIMIT 1`
		
    delete_post_str = `DELETE FROM posts WHERE (Id = ?1 OR Parent = ?1) AND Board = ?2`
    user_delete_str = `DELETE FROM posts WHERE Calendar >= ?1 AND Password = ?2 AND Board = ?3 AND 
	    ((SELECT COUNT(Id) FROM posts WHERE Parent = (SELECT Id FROM posts WHERE Password = ?2 AND Board = ?3 LIMIT 1) AND Board = ?3) <= 1)`    
		//threads with more than one post cannot be deleted by users
    filedelete_str = `UPDATE posts SET Imgprev = 'deleted', File = 'deleted', Filemime = 'image/webp' WHERE Id = ? and Board = ?`
    user_filedelete_str = `UPDATE posts SET Imgprev = 'deleted', File = 'deleted', Filemime = 'image/webp' WHERE Password = ? and Board = ?`
    delete_all_posts_str = `DELETE FROM posts WHERE (Identifier = (SELECT Identifier FROM posts WHERE Id = ?1 AND Board = ?2))`
    isparent_str = `SELECT IIF(Parent = Id, 1, 0) FROM posts WHERE Id = ? AND Board = ?`
    isparent_str2 = `SELECT IIF(Parent = Id, 1, 0), Id FROM posts WHERE Password = ? AND Board = ?`
    ban_str = `INSERT INTO banned(Identifier, Expiry, Mod, Content, Reason) VALUES ((SELECT Identifier FROM posts WHERE Id = ?1 AND Board = ?2), 
        ?3, ?4, (SELECT Content FROM posts WHERE Id = ?1 AND Board = ?2), ?5)`
    delete_log_str = `INSERT INTO deleted(Identifier, Time, Mod, Content, Reason) VALUES ((SELECT Identifier FROM posts WHERE Id = ?1 AND Board = ?2),
        ?3, ?4, replace(replace((SELECT Content FROM posts WHERE Id = ?1 AND Board = ?2), '<', '&lt;'), '>', '&gt;'), ?5)`
    ban_message_str = `UPDATE posts SET Content = Content || '<br><br><div class="banmessage">(' || ? || ')</div>' WHERE Id = ? AND Board = ?`
	
    get_deleted_str = `SELECT Identifier, Time FROM deleted`
    delete_remove_str = `DELETE FROM deleted WHERE Identifier = ? AND TIME = ?`
    get_expired_tokens_str = `SELECT Token, Time FROM tokens`
    delete_expired_token_str = `DELETE FROM tokens WHERE Token = ? AND TIME = ?`
    get_bans_str = `SELECT Identifier, Expiry FROM banned WHERE Expiry <> '-1'`
    delete_ban_str = `DELETE FROM banned WHERE Identifier || Expiry = ? || ?`

    lock_check_str = `SELECT COALESCE(Locked, 0) AS Locked FROM posts WHERE Parent = ? AND Board = ?`
    lock_str = `UPDATE posts SET Locked = 1 WHERE Id = ? AND Board = ?`
    unlock_str = `UPDATE posts SET Locked = 0 WHERE Id = ? AND Board = ?`
    pin_str = `UPDATE posts SET Pinned = 1 WHERE Id = ? AND Board = ?`
    unpin_str = `UPDATE posts SET Pinned = 0 WHERE Id = ? AND Board = ?`
)

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
    prep := func(SQL string) *sql.Stmt {
        conn, err := sql.Open("sqlite3", DB_uri)
        Err_check(err)
        stmt, err := conn.Prepare(SQL)
        Err_check(err)
        return stmt
    }
    
    for i := 0; i < Max_conns; i++ {

        //preview statements
        prev_stmt := prep(prev_str)
        prev_parentstmt := prep(prev_parent_str)

        //thread update statements
        updatestmt := prep(update_str)
        update_repstmt := prep(update_rep_str)

        //board upate statements
        parent_collstmt := prep(parent_coll_str)
        thread_headstmt := prep(thread_head_str)
        thread_bodystmt := prep(thread_body_str)

        //catalog update statement
        thread_collstmt := prep(thread_coll_str)
       
        //subject lookup
        subject_lookstmt := prep(subject_look_str)
        hp_collstmt := prep("SELECT Board, Id, Content, TrunContent, Parent, Password FROM homepost ORDER BY Insertorder DESC")
        ht_collstmt := prep("SELECT Board, Id, Parent, Imgprev, Password FROM homethumb ORDER BY Insertorder DESC LIMIT 6")
        shown_countstmt := prep(shown_count_str)
        total_countstmt := prep(total_count_str)
        rss_collstmt := prep(rss_coll_str)
        parent_checkstmt := prep(parent_check_str)
        
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
