package main

import (
    "database/sql"
    "os"
    "strings"

    _ "github.com/mattn/go-sqlite3"
)

var createSQL = [...]string {`CREATE TABLE posts (
        Insertorder INTEGER PRIMARY KEY ASC,
        Board TEXT NOT NULL,
        Id INTEGER NOT NULL,
        Content TEXT,
        Time TEXT,
        Parent INTEGER,
        Password TEXT NOT NULL,
        Identifier TEXT,
        File TEXT,
        Filename TEXT,
        Fileinfo TEXT,
        Filemime TEXT,
        Imgprev TEXT,
        Hash TEXT,
        Option TEXT,
        Calendar INTEGER NOT NULL,
        Clock INTEGER NOT NULL,
        Pinned INTEGER NOT NULL,
        Locked INTEGER NOT NULL,
        Anchored INTEGER NOT NULL,
        UNIQUE (Board, Id)
    );`,
    
    `CREATE VIRTUAL TABLE search USING fts5(Board, Id, Content, Time,
        content='posts', content_rowid='Insertorder');`,

    `CREATE TABLE replies (
        Board TEXT NOT NULL,
        Source INTEGER NOT NULL,
        Replier INTEGER NOT NULL,
        Password TEXT NOT NULL,
        FOREIGN KEY (Board, Replier) REFERENCES posts(Board, Id) ON DELETE CASCADE
    );`,


    `CREATE TABLE subjects (
        Board TEXT NOT NULL,
        Parent INTEGER NOT NULL,
        Subject TEXT NOT NULL
    );`,

    `CREATE TABLE latest (
        Board TEXT PRIMARY KEY,
        Id INTEGER NOT NULL
    );`,

    `CREATE TABLE homepost (
        Insertorder INTEGER PRIMARY KEY ASC,
        Board TEXT NOT NULL,
        Id INTEGER NOT NULL,
        Content TEXT NOT NULL,
        TrunContent TEXT NOT NULL,
        Parent INTEGER NOT NULL,
        Password TEXT NOT NULL,
        FOREIGN KEY (Board, Id) REFERENCES posts(Board, Id) ON DELETE CASCADE
    );`,

    `CREATE TABLE homethumb (
        Insertorder INTEGER PRIMARY KEY ASC,
        Board TEXT NOT NULL,
        Id INTEGER NOT NULL,
        Parent TEXT NOT NULL,
        Imgprev TEXT NOT NULL,
        Password TEXT NOT NULL,
        FOREIGN KEY (Board, Id) REFERENCES posts(Board, Id) ON DELETE CASCADE
    );`,

    `CREATE TABLE credentials (
        Username TEXT NOT NULL,
        Hash TEXT NOT NULL,
        Type INTEGER NOT NULL
    );`,

    `CREATE TABLE tokens (
        Token TEXT NOT NULL,
        Type TEXT NOT NULL,
        Time TEXT NOT NULL
    );`,

    `CREATE TABLE banned (
        Insertorder INTEGER PRIMARY KEY ASC,
        Identifier TEXT NOT NULL,
        Expiry TEXT NOT NULL,
        Mod TEXT NOT NULL,
        Content TEXT,
        Reason TEXT
    );`,

    `CREATE TABLE deleted (
        Identifier TEXT NOT NULL,
        Time TEXT NOT NULL,
        Mod TEXT NOT NULL,
        Content TEXT,
        Reason TEXT
    );`,

    //triggers
    `CREATE TRIGGER latest_update
        AFTER INSERT ON posts
        BEGIN
            UPDATE latest 
            SET Id = Id + 1
            WHERE Board = NEW.Board;
        END;`,

    `CREATE TRIGGER rep_clear
        AFTER UPDATE ON posts
        BEGIN
            DELETE FROM replies WHERE Replier = OLD.Id AND Board = OLD.Board;
            DELETE FROM homethumb WHERE Imgprev = OLD.Imgprev AND NEW.Imgprev = 'deleted';
        END;`,
        
    `CREATE TRIGGER anchor_check
        AFTER INSERT ON posts
        BEGIN
            UPDATE posts
            SET Anchored = IIF((SELECT COUNT(Id) FROM posts WHERE Parent = NEW.Parent AND Board = NEW.Board AND Pinned <> 1) > 200, 1, 0)
            WHERE Id = NEW.Parent AND Board = NEW.Board;
        END;`,
        
    `CREATE TRIGGER homepost_trim
        AFTER INSERT ON homepost
        BEGIN
            DELETE FROM homepost WHERE Insertorder =
                IIF((SELECT COUNT(Id) FROM homepost) > 20,
                (SELECT min(Insertorder) from homepost), NULL);
        END;`,

    `CREATE TRIGGER homethumb_trim
        AFTER INSERT ON homethumb
        BEGIN
            DELETE FROM homethumb WHERE Insertorder =
                IIF((SELECT COUNT(Id) FROM homethumb) > 10,
                (SELECT min(Insertorder) from homethumb), NULL);
        END;`,

    `CREATE TRIGGER posts_ai
        AFTER INSERT ON posts
        BEGIN
            INSERT INTO search(rowid, Board, Id, Content, Time)
                VALUES (new.Insertorder, new.Board, new.Id, new.Content, new.Time);
        END;`,


    `CREATE TRIGGER posts_ad
        AFTER DELETE ON posts
        BEGIN
            INSERT INTO search(search, rowid, Board, Id, Content, Time)
                VALUES ('delete', old.Insertorder, old.Board, old.Id, old.Content, old.Time);
        END;`,


    `CREATE TRIGGER posts_au
        AFTER UPDATE ON posts
        BEGIN
            INSERT INTO search(search, rowid, Board, Id, Content, Time)
                VALUES ('delete', old.Insertorder, old.Board, old.Id, old.Content, old.Time);

            INSERT INTO search(rowid, Board, Id, Content, Time)
                VALUES (new.Insertorder, new.Board, new.Id, new.Content, new.Time);
        END;`,
}

const (
    //how new posts know what their id is 
    latestseedSQL = `INSERT OR IGNORE INTO latest (Board, Id) VALUES (cb, 1);`
)

func create_table(db *sql.DB) {
    for _, stmt := range(createSQL) {
        statement, err := db.Prepare(stmt)
        Err_check(err)
        statement.Exec()
    }
}


func LatestSeed() {
    conn, err := sql.Open("sqlite3", DB_path)
    Err_check(err)
    defer conn.Close()
    
    for board := range Board_map {
        statement, err := conn.Prepare(strings.Replace(latestseedSQL, "cb", `'` + board + `'`, 1))
            Err_check(err)
        statement.Exec()
    }
}

func New_db() {

    file, err := os.Create(DB_path)
    Err_check(err)

    file.Close()

    conn, err := sql.Open("sqlite3", DB_path)
    Err_check(err)
    defer conn.Close()

    create_table(conn)
}
