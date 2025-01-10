package slug

import "strings"

func GenerateSlug(namaToko string) string {
    return strings.ToLower(strings.ReplaceAll(namaToko, " ", "-"))
}
