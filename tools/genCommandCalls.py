# -*- coding: utf-8 -*-

commands = {
    "Open" : ["str", "u32", "u32"],
    "Close" : ["u32"],
    "Read" : ["u32", "u64"],
    "ReadDir" : ["str"],
    "ReadLink" : ["str"],
    "Write" : ["u32", "data"],
    "Seek" : ["u32", "u8", "i64"],
    "Allocate" : ["u32", "u32", "u64", "u64"],
    "GetAttr" : ["str"],
    "SetAttr" : ["str", "u8", "u64", "i64", "u32", "u8"],
    "Sync" : ["u32"],
    "Mkdir" : ["str", "u32"],
    "SymLink" : ["str", "str"],
    "Remove" : ["str"],
    "RmDir" : ["str"],
    "FsStat" : ["str"],
    "ReadAt" : ["u32","u64", "u64"],
    "WriteAt" : ["u32", "u64","data"],
    "CopyFileRange": ["u32", "u32", "u64", "u64","u64"],
    "Rename": ["str", "str", "u32"],
    "SetAttrByFD" : ["u32", "u8", "u64", "i64", "u32", "u8"],
}

deltype = {
    "data": "= bufPool.Get().(*util.Buffer)",
    "str": "string",
    "u64": "uint64",
    "u32": "uint32",
    "u16": "uint16",
    "u8": "uint8",
    "i64": "int64",
    "i32": "int32",
    "i16": "int16",
    "i8": "int8",
}
    

for key, value in commands.items():
    print(f'case wsfsprotocol.Cmd{key}:')

    for i, v in enumerate(value):
        print(f'    var v{i} {deltype[v]}')
        if v == "data":
            print(f'    _, err = io.Copy(v{i}, r)')
        elif v == "str":
            print(f'    err = util.CopyStrFromReader(r, &v{i})')
        else:
            print(f'    err = binary.Read(r, binary.LittleEndian, &v{i})')
        #if i != len(value) - 1 :
        print(f'    if err != nil {{')
        #else:
        #    print(f'    if err != io.EOF {{')
        print(f'        goto BadCmd')
        print(f'    }}')

    print(f'    s.wg.Add(1)')
    print(f'    s.cmd{key}(clientMark, writeCh', end="")
    if len(value) != 0 :
        print(f', ', end="")
    for i, v in enumerate(value):
        print(f'v{i}', end="")
        if i != len(value) - 1 :
            print(f', ', end="")
    print(f')')


    