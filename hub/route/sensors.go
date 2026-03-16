package route

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/xjasonlyu/tun2socks/v2/log"
	"net/http"
)

func sensorRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getSensor)
	return r
}

func getSensor(w http.ResponseWriter, r *http.Request) {
	// 获取所有传感器的温度信息
	warnings, err := host.SensorsTemperatures()
	if err != nil {
		log.Debugf("获取失败: %v\n", err)
		return
	}

	for _, sensor := range warnings {
		log.Debugf("传感器: %-20s 温度: %.2f°C\n", sensor.SensorKey, sensor.Temperature)
	}

	render.JSON(w, r, render.M{
		"sensors": warnings,
	})
}
