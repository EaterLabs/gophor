package main

import (
    "os"
    "sync"
)

const (
    /* Filename constants */
    CapsTxtStr   = "caps.txt"
    RobotsTxtStr = "robots.txt"
)

func cachePolicyFiles(description, admin, geoloc string) {
    /* See if caps txt exists, if not generate */
    _, err := os.Stat("/caps.txt")
    if err != nil {
        /* We need to generate the caps txt and manually load into cache */
        content := generateCapsTxt(description, admin, geoloc)

        /* Create new file object from generated file contents */
        fileContents := &GeneratedFileContents{ content }
        file := &File{ fileContents, sync.RWMutex{}, true, 0 }

        /* Trigger a load contents just to set it as fresh etc */
        file.LoadContents()

        /* No need to worry about mutexes here, no other goroutines running yet */
        Config.FileSystem.CacheMap.Put("/"+CapsTxtStr, file)
    }

    /* See if caps txt exists, if not generate */
    _, err = os.Stat("/robots.txt")
    if err != nil {
        /* We need to generate the caps txt and manually load into cache */
        content := generateRobotsTxt()

        /* Create new file object from generated file contents */
        fileContents := &GeneratedFileContents{ content }
        file := &File{ fileContents, sync.RWMutex{}, true, 0 }

        /* Trigger a load contents just to set it as fresh etc */
        file.LoadContents()

        /* No need to worry about mutexes here, no other goroutines running yet */
        Config.FileSystem.CacheMap.Put("/"+RobotsTxtStr, file)
    }
}

func generateCapsTxt(description, admin, geoloc string) []byte {
    text := "CAPS"+DOSLineEnd
    text += DOSLineEnd
    text += "# This is an automatically generated"+DOSLineEnd
    text += "# server policy file: caps.txt"+DOSLineEnd
    text += DOSLineEnd
    text += "CapsVersion=1"+DOSLineEnd
    text += "ExpireCapsAfter=1800"+DOSLineEnd
    text += DOSLineEnd
    text += "PathDelimeter=/"+DOSLineEnd
    text += "PathIdentity=."+DOSLineEnd
    text += "PathParent=.."+DOSLineEnd
    text += "PathParentDouble=FALSE"+DOSLineEnd
    text += "PathEscapeCharacter=\\"+DOSLineEnd
    text += "PathKeepPreDelimeter=FALSE"+DOSLineEnd
    text += DOSLineEnd
    text += "ServerSoftware=Gophor"+DOSLineEnd
    text += "ServerSoftwareVersion="+GophorVersion+DOSLineEnd
    text += "ServerDescription="+description+DOSLineEnd
    text += "ServerGeolocationString="+geoloc+DOSLineEnd
//    text += "ServerDefaultEncoding=ascii"+DOSLineEnd
    text += DOSLineEnd
    text += "ServerAdmin="+admin+DOSLineEnd
    return []byte(text)
}

func generateRobotsTxt() []byte {
    text := "Usage-agent: *"+DOSLineEnd
    text += "Disallow: *"+DOSLineEnd
    text += DOSLineEnd
    text += "Crawl-delay: 99999"+DOSLineEnd
    text += DOSLineEnd
    text += "# This server does not support scraping"+DOSLineEnd
    return []byte(text)
}
