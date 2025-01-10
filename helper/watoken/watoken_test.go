package watoken

import (
	"os"
	"testing"
)

func TestEncode(t *testing.T) {
	privkey := os.Getenv("PRIVATEKEY")
	str, _ := EncodeforHours(os.Getenv("PHONENUMBER"), "Helpdesk Pamong Desa", privkey, 43830)
	println(str)
	//atr, _ := DecodeGetId("", str)
	//println(atr)

}
