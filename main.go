package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xuqingfeng/bond"
)

func main() {

	go bond.Process()

	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hey there!")
	})

	e.POST("/fetch", func(c echo.Context) error {
		err := bond.FetchFutureBonds()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusOK, "OK")
	})

	// 上市提醒 + 打新提醒
	e.POST("/notify", func(c echo.Context) error {
		err := bond.Notify()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusOK, "OK")
	})

	e.Logger.Fatal(e.Start(":8000"))
}
