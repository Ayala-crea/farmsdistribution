package controller

import (
	"farmdistribution_be/helper/at"
	"farmdistribution_be/model"
	"net/http"
)

func GetHome(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	resp.Response = at.GetIPaddress()
	at.WriteJSON(respw, http.StatusOK, resp)
}
