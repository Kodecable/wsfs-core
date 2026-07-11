"use strict"
var GLock = 0
var GTableHeaderElements = {}
const GTableHeaderCellIndex = { "name": 0, "size": 1, "time": 2 }
var GItemsElement
var GProgressLastUpdateTime
var GProgressLastUpdateValue = 0
var GProgressAvgSpeed = 0
var GDragCounter = 0

function JLock() { GLock += 1; if (GLock == 1) { return false } else { GLock -= 1; return true } }
function JUnlock() { GLock -= 1 }
function Exists(o) { return (typeof (o) != 'undefined' && o != null) }
function $(s) { return document.querySelector(s) }


function GracefullyLoad(url, pushState = true) {
    let xhr = new XMLHttpRequest()
    xhr.open("GET", url, true)
    xhr.responseType = "document"
    xhr.onloadend = function () {
        if (xhr.readyState != xhr.DONE
            || xhr.status != 200
            || !Exists(xhr.responseXML)
            || (GCacheId != xhr.responseXML.querySelector("body").dataset.cacheid)) {
            location.href = url
            return
        }
        if (pushState)
            history.pushState({}, "", url);
        $(".path").remove()
        $("main").children[0].before(xhr.responseXML.querySelector(".path"))
        $("#files>tbody").remove()
        $("#files>thead").after(xhr.responseXML.querySelector("#files>tbody"))
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
    $("main").insertAdjacentHTML("beforeend", `<div id="PopupDialogBackground"><div id="ProgressDialog" class="dialog column"><div class="row"><h1 id="ProgrssTitle">${title}</h1><h1 id="ProgressDesc"></h1></div><div id="Progress"><div id="ProgressValue"></div></div><div class="row row-separation"><p id="ProgressPercent">0%</p><p id="ProgressETA"></p></div></div></div>`)
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

function StableSpeed(newspeed, timeInterval) {
    const halfLife = 1000; // ms
    
    if (GProgressAvgSpeed === 0 || isNaN(GProgressAvgSpeed)) {
        GProgressAvgSpeed = newspeed;
        return GProgressAvgSpeed;
    }
    
    const decayFactor = Math.pow(0.5, timeInterval / halfLife);
    const newWeight = 1 - decayFactor;
    const oldWeight = decayFactor;
    GProgressAvgSpeed = newWeight * newspeed + oldWeight * GProgressAvgSpeed;
    return GProgressAvgSpeed;
}

function UpdateProgressDialog(desc, totalValue, value) {
    let now = performance.now()
    $("#ProgressDesc").innerHTML = desc
    $("#ProgressValue").style.width = (value / totalValue * 100).toString() + "%"
    $("#ProgressPercent").innerHTML = Math.floor(value / totalValue * 100).toString() + "%"
    if (GProgressLastUpdateValue != 0 && value != totalValue) {
        let timeInterval = now - GProgressLastUpdateTime
        let speed = (value - GProgressLastUpdateValue) / timeInterval
        speed = StableSpeed(speed, timeInterval)
        $("#ProgressETA").innerHTML = FormatSeconds((totalValue - value) / speed / 1000)
    }
    GProgressLastUpdateValue = value
    GProgressLastUpdateTime = now
}

function LogProgressDialog(str) {
    let logger = $("#ProgressLogger")
    if (!Exists(logger)) {
        $("#ProgressDialog").innerHTML += '<p id="ProgressLogger" style="color: red"></p>'
        logger = $("#ProgressLogger")
    }
    logger.innerHTML += str + "\n"
}

function CloseProgressDialog() {
    $("#ProgressDesc").remove()
    $("#Progress").remove()
    $("#ProgressPercent").parentElement.remove()
    if (Exists($("#ProgressLogger"))) {
        $("#ProgrssTitle").innerHTML = I18nText("Error(s) occurred")
        return
    }
    GracefullyLoad(".", false)
    $("#PopupDialogBackground").remove()
}

function OpenFileInputer() {
    if (JLock()) return
    document.getElementById('FileInputer').click()
}

function StartUpload(files) {
    if (files.length < 1) {
        JUnlock()
        return
    }
    // Get the file before changing the dom
    // After changing the dom the selected file seems to disappear
    OpenProgressDialog(I18nText("Uploading"))
    let fullSize = 0
    for (const file of files)
        fullSize += file.size
    UploadFile(files, fullSize, 0, 0)
}

function UploadFiles() {
    const files = document.getElementById("FileInputer").files
    StartUpload(files)
}

function ShowDropMask() {
    $("#DropUploadMask").classList.add("Show")
}

function HideDropMask() {
    GDragCounter = 0
    $("#DropUploadMask").classList.remove("Show")
}

function IsFileDrag(event) {
    return Exists(event.dataTransfer)
        && Exists(event.dataTransfer.types)
        && event.dataTransfer.types.includes("Files")
}

function DragUploadEnter(event) {
    if (!IsFileDrag(event)) return
    event.preventDefault()
    GDragCounter += 1
    ShowDropMask()
}

function DragUploadOver(event) {
    if (!IsFileDrag(event)) return
    event.preventDefault()
}

function DragUploadLeave(event) {
    if (!IsFileDrag(event)) return
    event.preventDefault()
    GDragCounter -= 1
    if (GDragCounter <= 0) HideDropMask()
}

function DragUploadDrop(event) {
    if (!IsFileDrag(event)) return
    event.preventDefault()
    HideDropMask()
    if (JLock()) {
        alert(I18nText("Uploading"))
        return
    }
    const files = event.dataTransfer.files
    StartUpload(files)
}

function InitDragUpload() {
    document.addEventListener("dragenter", DragUploadEnter)
    document.addEventListener("dragover", DragUploadOver)
    document.addEventListener("dragleave", DragUploadLeave)
    document.addEventListener("drop", DragUploadDrop)
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
        let cmp = 0;
        let isDir1 = o1.classList.contains("dirItem");
        let isDir2 = o2.classList.contains("dirItem");

        if (field == "time") {
            let t1 = Date.parse(o1.children[index].getAttribute("title"));
            let t2 = Date.parse(o2.children[index].getAttribute("title"));
            if (isNaN(t1)) t1 = -1;
            if (isNaN(t2)) t2 = -1;
            cmp = t1 - t2;
        } else if (field == "size") {
            if (isDir1 != isDir2) return isDir1 ? -1 : 1;
            let s1 = parseInt(o1.children[index].getAttribute("title"));
            let s2 = parseInt(o2.children[index].getAttribute("title"));
            if (isNaN(s1)) s1 = -1;
            if (isNaN(s2)) s2 = -1;
            cmp = s1 - s2;
        }
        if (cmp != 0) return cmp * sortFactor;

        // fallback sort for size / time
        let nameSortFactor = sortFactor
        if (field != "name") nameSortFactor = 1;
        
        if (isDir1 != isDir2) return isDir1 ? -1 : 1;
        return nameSortFactor * (o1.children[index].innerHTML.localeCompare(o2.children[index].innerHTML));
    })
    for (let value of items) {
        GItemsElement.appendChild(value)
    }

    GTableHeaderElements[field].classList.add(sortFactor == -1 ? "SortAsc" : "SortDes")
    JUnlock()
}

function initWebui(reload) {
    if (!reload) {
        if(!GReadOnly) {
            $("main>.path").insertAdjacentHTML("afterend", `<div class="row">
		    <button type="button" onclick="OpenFileInputer()">
			    <span class="icon uploadFileIcon"></span><div>${I18nText("Upload")}</div>
		    </button>
		    <button type="button" onclick="NewFolder()">
			    <span class="icon newFolderIcon"></span><div>${I18nText("New folder")}</div>
		    </button>
	        </div>`)
            $("main").insertAdjacentHTML("afterend", "<input type='file' id='FileInputer' style='opacity:0' onchange='UploadFiles()' multiple />")
            $("main").insertAdjacentHTML("beforeend", `<div id="DropUploadMask"><div>${I18nText("Drop files to upload")}</div></div>`)
            $("#FileInputer").addEventListener("cancel", () => { JUnlock() })
            InitDragUpload()
        }
        for (const field of ["name", "size", "time"]) {
            GTableHeaderElements[field] = document.getElementById(`${field}Header`)
            GTableHeaderElements[field].addEventListener("click", () => { Sort(field) })
            GTableHeaderElements[field].style = "cursor: pointer"
        }
    }
    GItemsElement = $("#files>tbody")
    for (let elem of GItemsElement.children) {
        elem.setAttribute("title", elem.getElementsByTagName("a")[0].textContent)
        // convert to local time
        let d = new Date(elem.children[2].getAttribute("title"))
        let pad = (n) => String(n).padStart(2, "0")
        elem.children[2].innerHTML = `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
    }
    GItemsElement.addEventListener("click", ElementAGracefullyLoad)
    $(".path").addEventListener("click", ElementAGracefullyLoad)
    Sort("name", 1)
}

window.addEventListener("load", () => { initWebui(false) })
