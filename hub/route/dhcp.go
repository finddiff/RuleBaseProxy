package route

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/xjasonlyu/tun2socks/v2/log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	dhcpd_leases "github.com/npotts/go-dhcpd-leases"
)

func dhcpRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getDHCP)

	return r
}

func ipv6Neighbour() map[string][]string {
	neighbour_map := make(map[string][]string)
	cmd := exec.Command("sudo", "ip", "-6", "neighbour", "show")

	// 执行命令并获取标准输出和标准错误的组合输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("ipv6Neighbour error: %v", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		items := strings.Split(line, " ")
		log.Debugf("ipv6Neighbour items: %d", len(items))
		if len(items) != 7 {
			continue
		}
		log.Debugf("ipv6: %s , mac:%s \n", items[0], items[4])
		ipv6 := items[0]
		mac := items[4]
		if iplist, exists := neighbour_map[mac]; exists {
			neighbour_map[mac] = append(iplist, ipv6)
		} else {
			neighbour_map[mac] = []string{ipv6}
		}
	}

	return neighbour_map
}

func getDHCP(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat("/var/lib/dhcp/dhcpd.leases"); os.IsNotExist(err) {
		return
	}

	f, err := os.Open("/var/lib/dhcp/dhcpd.leases")
	if err != nil {
		return
	}
	defer f.Close()

	leases := dhcpd_leases.Parse(f)
	new_leases := []dhcpd_leases.Lease{}
	new_leases_map := make(map[string]dhcpd_leases.Lease)

	currentTime := time.Now()
	for _, item := range leases {
		if item.Ends.After(currentTime) {
			new_leases_map[item.Hardware.MAC] = item
		}
	}

	ipv6_neighbour := ipv6Neighbour()
	for _, item := range new_leases_map {
		mac := item.Hardware.MAC
		if iplist, exists := ipv6_neighbour[mac]; exists {
			item.Hardware.MAC += "," + strings.Join(iplist, ",")
		}
		new_leases = append(new_leases, item)
	}

	render.JSON(w, r, render.M{
		"dhcpd_leases": new_leases,
	})
}
