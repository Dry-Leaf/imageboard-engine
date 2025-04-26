package main

import (
    "time"
    "net/http"
)

func Switch_theme(w http.ResponseWriter, req *http.Request) {
    if Request_filter(w, req, "GET", 1 << 13) == 0 {return}

    cookie := &http.Cookie{
            Name:   "theme",
            Value:  req.FormValue("theme"),
        Expires: time.Now().AddDate(10, 0, 0),
            Path: "/",
        }

    w.Header().Set("Clear-Site-Data", "\"cache\"")
    http.SetCookie(w, cookie)    

    http.Redirect(w, req, req.Header.Get("Referer"), 302)
}
