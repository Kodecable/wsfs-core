"use strict"
var GLock = 0
var GTableHeaderElements = {}
const GTableHeaderCellIndex = { "name": 0, "size": 1, "time": 2 }
var GItemsElement
var GProgressLastUpdateTime
var GProgressLastUpdateValue = 0
var GProgressAvgSpeed = 0

function JLock() { GLock += 1; if (GLock == 1) { return false } else { GLock -= 1; return true } }
function JUnlock() { GLock -= 1 }
function Exists(o) { return (typeof (o) != 'undefined' && o != null) }
//function GetValue(id) { return document.getElementById(id).value }

function UnhumanizeSize(text) {
    const powers = { 'k': 1, 'm': 2, 'g': 3, 't': 4, 'p': 5, 'e': 6, 'z': 7, 'y': 8 }
    const regex = /(\d+(?:\.\d+)?)\s?(k|m|g|t|p|e|z|y)?i?b?/i

    let res = regex.exec(text)
    if (!Exists(res) || !Exists(res[1])) return -1
    if (!Exists(res[2])) return Number(res[1])
    return res[1] * Math.pow(1024, powers[res[2].toLowerCase()])
}

function GracefullyLoad(url, pushState = true) {
    let xhr = new XMLHttpRequest()
    xhr.open("GET", url, true)
    xhr.responseType = "document"
    xhr.onloadend = function () {
        if (xhr.readyState != xhr.DONE
            || xhr.status != 200
            || !Exists(xhr.responseXML)
            || (GCacheId != xhr.responseXML.getElementsByTagName("body")[0].dataset.cacheid)) {
            location.href = url
            return
        }
        if (pushState)
            history.pushState({}, "", url);
        document.getElementsByClassName("path")[0].remove()
        document.getElementsByTagName("main")[0].children[0].before(xhr.responseXML.getElementsByClassName("path")[0])
        document.getElementById("files").getElementsByTagName("tbody")[0].remove()
        document.getElementById("files").getElementsByTagName("thead")[0].after(xhr.responseXML.getElementById("files").getElementsByTagName("tbody")[0])
        initWebui(true)
    }
    xhr.send()
}
window.onpopstate = function () {
    GracefullyLoad(location.href, false)
}

function ElementAGracefullyLoad(event) {
    if (!Exists(event.target.tagName)
        || event.target.tagName.toLowerCase() != "a"
        || !event.target.getAttribute("href").endsWith("/"))
        return
    event.preventDefault()
    GracefullyLoad(event.target.getAttribute("href"))
}

function OpenProgressDialog(title) {
    document.getElementsByTagName("main")[0].innerHTML += `<div id="PopupDialogBackground"><div id="ProgressDialog" class="dialog column"><div class="row"><h1 id="ProgrssTitle">${title}</h1><h1 id="ProgressDesc"></h1></div><div id="Progress"><div id="ProgressValue"></div></div><div class="row row-separation"><p id="ProgressPercent">0%</p><p id="ProgressETA"></p></div></div></div>`
}

function FormatSeconds(v) {
    if (v > 24 * 60 * 60) return I18nText("More than 1 day")
    if (v < 0) return ""
    if (v < 1) return "1"
    v = Math.round(v)
    let str = (v % 60).toString()
    v = Math.floor(v / 60)
    if (v < 1)
        return str
    if (str.length == 1)
        str = `0${str}`
    str = `${(v % 60)}:${str}`
    v = Math.floor(v / 60)
    if (v < 1)
        return str
    if (str.length == 4)
        str = `0${str}`
    str = `${(v % 60)}:${str}`
    return str
}

function StableSpeed(newspeed) {
    let diff = Math.abs(newspeed - GProgressAvgSpeed) / GProgressAvgSpeed
    if (diff < 0.5)
        return GProgressAvgSpeed = 0.05 * newspeed + 0.95 * GProgressAvgSpeed
    else if (diff < 0.7)
        return GProgressAvgSpeed = 0.1 * newspeed + 0.9 * GProgressAvgSpeed
    else if (diff < 0.8)
        return GProgressAvgSpeed = 0.2 * newspeed + 0.8 * GProgressAvgSpeed
    else if (diff < 0.9)
        return GProgressAvgSpeed = 0.3 * newspeed + 0.7 * GProgressAvgSpeed
    else if (diff < 1)
        return GProgressAvgSpeed = 0.5 * newspeed + 0.5 * GProgressAvgSpeed
    return GProgressAvgSpeed = newspeed
}

function UpdateProgressDialog(desc, totalValue, value) {
    let now = performance.now()
    document.getElementById("ProgressDesc").innerHTML = desc
    document.getElementById("ProgressValue").style.width = (value / totalValue * 100).toString() + "%"
    document.getElementById("ProgressPercent").innerHTML = Math.floor(value / totalValue * 100).toString() + "%"
    if (GProgressLastUpdateValue != 0 && value != totalValue) {
        let speed = (value - GProgressLastUpdateValue) / (now - GProgressLastUpdateTime)
        speed = StableSpeed(speed)
        document.getElementById("ProgressETA").innerHTML = FormatSeconds((totalValue - value) / speed / 1000)
    }
    GProgressLastUpdateValue = value
    GProgressLastUpdateTime = now
}

function LogProgressDialog(str) {
    let logger = document.getElementById("ProgressLogger")
    if (!Exists(logger)) {
        document.getElementById("ProgressDialog").innerHTML += '<p id="ProgressLogger" style="color: red"></p>'
        logger = document.getElementById("ProgressLogger")
    }
    logger.innerHTML += str + "\n"
}

function CloseProgressDialog() {
    document.getElementById("ProgressDesc").remove()
    document.getElementById("Progress").remove()
    document.getElementById("ProgressPercent").parentElement.remove()
    if (Exists(document.getElementById("ProgressLogger"))) {
        document.getElementById("ProgrssTitle").innerHTML = I18nText("Error(s) occurred")
        return
    }
    GracefullyLoad(".", false)
    document.getElementById("PopupDialogBackground").remove()
}

function OpenFileInputer() {
    if (JLock()) return
    document.getElementById('FileInputer').click()
}

function UploadFiles() {
    const files = document.getElementById("FileInputer").files
    // Get the file before changing the dom 
    // After changing the dom the selected file seems to disappear
    OpenProgressDialog(I18nText("Uploading"))
    let fullSize = 0
    for (const file of files)
        fullSize += file.size
    UploadFile(files, fullSize, 0, 0)
}

// onProgess(int uploaded)
// onEnd(bool ok, string msg) msg should be ignored if ok is true
function UploadFileChunk(file, chunkI, retryCount, onProgress, onEnd) {
    const chunkSize = 64 * 1024 * 1024 // 64 MiB
    const retryMax = 3
    let last = file.size - chunkI * chunkSize < chunkSize

    let xhr = new XMLHttpRequest();
    xhr.open(chunkI == 0 ? "PUT" : "PATCH", window.location.href + file.name, true)
    if (chunkI == 0)
        xhr.setRequestHeader("Overwrite", "T")
    else {
        xhr.setRequestHeader("Content-Type", "application/x-sabredav-partialupdate")
        xhr.setRequestHeader("X-Update-Range", `bytes=${chunkI * chunkSize}-`)
    }
    xhr.upload.onprogress = (e) => { onProgress(chunkI * chunkSize + e.loaded) }
    xhr.onloadend = () => {
        if (xhr.readyState != xhr.DONE
            || xhr.status > 299
            || xhr.status < 200) {
            if (retryCount >= retryMax)
                onEnd(false, xhr.status != 0 ? xhr.status : "connection")
            else
                UploadFileChunk(file, chunkI, retryCount + 1, onProgress, onEnd)
        } else {
            if (last)
                onEnd(true, "")
            else
                UploadFileChunk(file, chunkI + 1, 0, onProgress, onEnd)
        }
    }
    xhr.send(file.slice(chunkI * chunkSize, last ? file.size : (chunkI + 1) * chunkSize))
}

function UploadFile(files, fullSize, fullUploadedSize, i) {
    if (files.length == i) {
        CloseProgressDialog()
        JUnlock()
        return
    }
    UploadFileChunk(files.item(i), 0, 0,
        (uploaded) => {
            UpdateProgressDialog(files.item(i).name, fullSize, fullUploadedSize + uploaded)
        }, (ok, msg) => {
            if (!ok)
                LogProgressDialog(`Upload failed(${msg}): ${files.item(i).name}`)
            UploadFile(files, fullSize, fullUploadedSize + files.item(i).size, i + 1)
        })
}

function NewFolder() {
    if (JLock()) return
    let name = prompt(I18nText("New folder name:"), "new folder")
    if (!Exists(name)) return
    OpenProgressDialog(I18nText("Creating"))
    UpdateProgressDialog(name, 1, 0)
    let xhr = new XMLHttpRequest()
    xhr.open("MKCOL", window.location.href + name + "/", true)
    xhr.onloadend = () => {
        if (xhr.readyState != xhr.DONE
            || xhr.status > 299
            || xhr.status < 200)
            LogProgressDialog(`Create folder failed(${xhr.status != 0 ? xhr.status : "connection"}): ${files.item(i).name}`)
        CloseProgressDialog()
        JUnlock()
    }
    xhr.send()
}

function Sort(field, factor) {
    if (JLock()) return;

    let lastField = ""
    let sortFactor = 1;
    for (const [key, value] of Object.entries(GTableHeaderElements)) {
        if (value.classList.contains("SortAsc")) {
            sortFactor = -1
            lastField = key
        }
        else if (value.classList.contains("SortDes")) {
            sortFactor = 1
            lastField = key
        }
        value.classList.remove("SortAsc", "SortDes")
    }
    if (field == lastField) sortFactor *= -1
    if (Exists(factor)) sortFactor = factor

    let items = Array.from(GItemsElement.children)
    let index = GTableHeaderCellIndex[field]
    items.sort(function (o1, o2) {
        let n1 = o1.children[index].innerHTML;
        let n2 = o2.children[index].innerHTML;
        if (field == "size")
            return (UnhumanizeSize(n1) - UnhumanizeSize(n2)) * sortFactor
        return ((o1.classList.contains("dirItem") ? "dir" : "file" + n1).
            localeCompare(o2.classList.contains("dirItem") ? "dir" : "file" + n2)) * sortFactor;
    })
    for (let value of items) {
        GItemsElement.appendChild(value)
    }

    GTableHeaderElements[field].classList.add(sortFactor == -1 ? "SortAsc" : "SortDes")
    JUnlock()
}

function initWebui(reload) {
    if (!reload) {
        for (const field of ["name", "size", "time"]) {
            GTableHeaderElements[field] = document.getElementById(`${field}Header`)
            GTableHeaderElements[field].addEventListener("click", function () { Sort(field) })
            GTableHeaderElements[field].style = "cursor: pointer"
        }
        document.getElementById("FileInputer").addEventListener("cancel", () => {JUnlock()})
    }
    GItemsElement = document.getElementById("files").getElementsByTagName("tbody")[0];
    document.getElementById("files").getElementsByTagName("tbody")[0].addEventListener("click", ElementAGracefullyLoad)
    document.getElementsByClassName("path")[0].addEventListener("click", ElementAGracefullyLoad)
    Sort("name", 1)
}

window.addEventListener("load", () => { initWebui(false) })