"use strict"

let GLocale = {}

function I18nText(text) {
    let ttext = GLocale[text]
    return ttext != undefined ? ttext : text
}

function I18nElement(element) {
    let ttext = GLocale[element.innerHTML]
    if (ttext != undefined) element.innerHTML = ttext
}

{
    const locales = {
        "zh": {
            "Name": "名称",
            "Size": "大小",
            "Modification Time": "修改时间",
            "Error(s) occurred": "发生错误",
            "New folder name": "新文件夹名：",
            "Creating": "创建中",
            "Uploading": "上传中",
            "More than 1 day": "超过 1 天",
            "Upload": "上传文件",
            "New folder": "新建文件夹",
            "Return to root": "回到根目录",
            "Not found": "资源不存在",
            "Forbidden": "拒绝访问",
        },
    };
    for (const [key, value] of Object.entries(locales))
        if (window.navigator.language.toLowerCase().includes(key)) {
            GLocale = value
            document.querySelectorAll('[data-t]').forEach((element) => {
                I18nElement(element)
                element.removeAttribute("data-t")
            })
            break
        }
}