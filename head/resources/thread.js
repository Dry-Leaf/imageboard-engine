const plinks = document.querySelectorAll('a.plink');

plinks.forEach(plink => plink.addEventListener('click', AddIdToNewPost, false));

function AddIdToNewPost(e) {
    let id = e.target.innerText;
    document.getElementById('newpost').value += '>>' + id + '\n';
}


const replies = document.querySelectorAll('a.preview');

replies.forEach(reply => 
    reply.addEventListener('mouseover', GetPostData, 
    {once : true, capture: false})
);

async function GetPostData (e) {
    let url = e.target.getAttribute('prev-get');
    let response = await fetch(url)
    response.text().then((text) => {
        e.target.insertAdjacentHTML('afterend', 
            '<box class="prev">' + text + '</box>')
    });
}


const context_menu_html = `<div id="contextMenu" class="context-menu"> 
    <ul> 
        <a id="oiint" draggable="false" href="" target="_blank" class="oiint"><li>Open Image in New Tab</li></a>
        <a id="sia" draggable="false" href="" download="" class="oiint"><li>Save Image As...</li></a>
        <li id="cil" text="">Copy Image Link</li>
    </ul> 
</div>`;

document.onload = document.querySelector('p.headerblock').insertAdjacentHTML("beforebegin", context_menu_html);

const context_menu = document.getElementById("contextMenu");
const oiint = document.getElementById("oiint");
const sia = document.getElementById("sia");
const cil = document.getElementById("cil");
var menu_state = 0;

const url_regex = /url\("([^"]+)"\)/;

function ImContext (e) {
    if (event.target.classList.contains("image")) {
        e.preventDefault();

        image_style = window.getComputedStyle(e.target);
        image_url = url_regex.exec(image_style.getPropertyValue("content"))[1];
        image_name = image_style.getPropertyValue('--name');

        oiint.setAttribute("href", image_url);
        sia.setAttribute("href", image_url);
        if (event.target.checked) {
            sia.setAttribute("download", image_name);
        } else {sia.setAttribute("download", "");}

        cil.setAttribute("text", image_url);

        context_menu.style.left = e.pageX + "px"; 
        context_menu.style.top = e.pageY + "px"; 

        context_menu.style.display = 'block'; 
    } else {HideMenu();}
}

function CopyImLink() {
    navigator.clipboard.writeText(this.getAttribute("text"));
}
cil.addEventListener("click", CopyImLink);


function HideMenu () {
    context_menu.style.display = 'none';
}

window.onkeyup = function (e) {
    if (e.keyCode === 27) {
        HideMenu();
}}
window.onblur = function () {HideMenu();}

document.addEventListener("contextmenu", ImContext);
document.addEventListener("click", HideMenu);
