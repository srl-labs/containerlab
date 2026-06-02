# version

## Description

The `version` command displays containerlab's version information.

## Usage

```
containerlab version
  ____ ___  _   _ _____  _    ___ _   _ _____ ____  _       _     
 / ___/ _ \| \ | |_   _|/ \  |_ _| \ | | ____|  _ \| | __ _| |__  
| |  | | | |  \| | | | / _ \  | ||  \| |  _| | |_) | |/ _` | '_ \ 
| |__| |_| | |\  | | |/ ___ \ | || |\  | |___|  _ <| | (_| | |_) |
 \____\___/|_| \_| |_/_/   \_\___|_| \_|_____|_| \_\_|\__,_|_.__/ 

    version: 0.69.3
     commit: 49ee599b
       date: 2025-08-06T21:02:24Z
     source: https://github.com/srl-labs/containerlab
 rel. notes: https://containerlab.dev/rn/0.69/#0693
```

## Flags

### short

With `--short | -s` flag, only the version number is displayed.

### json

With `--json | -j` flag, the version information is displayed in JSON format.

```
clab version -j
{"version":"0.0.0","commit":"none","date":"unknown","repository":"https://github.com/srl-labs/containerlab","releaseNotes":"https://containerlab.dev/rn/0.0/"}
```
