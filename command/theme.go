package main

import (
    "time"
    "net/http"
    "context"
)

func Switch_theme(w http.ResponseWriter, req *http.Request) {
    //time out
    _, cancel := context.WithTimeout(req.Context(), 10 * time.Millisecond)
    defer cancel()

    if Request_filter(w, req, "GET", 1 << 13) == 0 {return}

    cookie := &http.Cookie{
            Name:   "theme",
            Value:  req.FormValue("theme"),
        Expires: time.Now().AddDate(10, 0, 0),
            Path: "/",
        }

    http.SetCookie(w, cookie)    

    http.Redirect(w, req, req.Header.Get("Referer"), 302)
}
