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

const thumbs = document.querySelectorAll('input.image');

thumbs.forEach(thumb => thumb.addEventListener('contextmenu', ImContext, false));

const url_regex = /url\("([^"]+)"\)/;

function ImContext(e) {
    event.preventDefault();
    image_style = window.getComputedStyle(e.target);
    imgsrc = url_regex.exec(image_style.getPropertyValue("content"))[1];

    window.open(imgsrc, '_blank');
}
