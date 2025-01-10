package config

import "os"

var PUBLICKEY string = "47db091df55d64499fdf2ca85504ac4d320c4c645c9bef75efac0494314cae94"

var GHAccessToken string = os.Getenv("GH_ACCESS_TOKEN")
