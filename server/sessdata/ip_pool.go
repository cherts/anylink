package sessdata

import (
	"net"
	"sync"
	"time"

	"github.com/cherts/anylink/base"
	"github.com/cherts/anylink/dbdata"
	"github.com/cherts/anylink/pkg/utils"
)

var (
	IpPool   = &ipPoolConfig{}
	ipActive = map[string]bool{}
	// ipKeep and ipLease  ipAddr => macAddr
	// ipKeep    = map[string]string{}
	ipPoolMux sync.Mutex
)

type ipPoolConfig struct {
	// Calculate dynamic IP
	Ipv4Gateway net.IP
	Ipv4Mask    net.IP
	Ipv4IPNet   *net.IPNet
	IpLongMin   uint32
	IpLongMax   uint32
}

func initIpPool() {

	// Address processing
	_, ipNet, err := net.ParseCIDR(base.Cfg.Ipv4CIDR)
	if err != nil {
		panic(err)
	}
	IpPool.Ipv4IPNet = ipNet
	IpPool.Ipv4Mask = net.IP(ipNet.Mask)

	ipv4Gateway := net.ParseIP(base.Cfg.Ipv4Gateway)
	ipStart := net.ParseIP(base.Cfg.Ipv4Start)
	ipEnd := net.ParseIP(base.Cfg.Ipv4End)
	if !ipNet.Contains(ipv4Gateway) || !ipNet.Contains(ipStart) || !ipNet.Contains(ipEnd) {
		panic("IP segment setting error")
	}
	// IP address pool
	IpPool.Ipv4Gateway = ipv4Gateway
	IpPool.IpLongMin = utils.Ip2long(ipStart)
	IpPool.IpLongMax = utils.Ip2long(ipEnd)

	loopCurIp = IpPool.IpLongMin
}

func ipInPool(ip net.IP) bool {
	if utils.Ip2long(ip) >= IpPool.IpLongMin && utils.Ip2long(ip) <= IpPool.IpLongMax {
		return true
	}
	return false
}

// AcquireIp Get dynamic ip
func AcquireIp(username, macAddr string, uniqueMac bool) (newIp net.IP) {
	base.Trace("AcquireIp start:", username, macAddr, uniqueMac)
	ipPoolMux.Lock()
	defer func() {
		ipPoolMux.Unlock()
		base.Trace("AcquireIp end:", username, macAddr, uniqueMac, newIp)
		base.Info("AcquireIp ip:", username, macAddr, uniqueMac, newIp)
	}()

	var (
		err  error
		tNow = time.Now()
	)

	// Priority check: IP reservation (Username)
	userKeepMaps := []dbdata.IpMap{}
	err = dbdata.FindWhere(&userKeepMaps, 10, 1, "username=? AND keep_username=?", username, true)
	if err == nil && len(userKeepMaps) > 0 {
		for _, mi := range userKeepMaps {
			ipStr := mi.IpAddr
			ip := net.ParseIP(ipStr)
			if _, ok := ipActive[ipStr]; !ok && ipInPool(ip) {
				mi.LastLogin = tNow
				if macAddr != "" {
					// If the new MAC already had an old non-backup entry, delete it to avoid a unique(mac_addr) conflict.
					oldMac := &dbdata.IpMap{}
					if errOld := dbdata.One("mac_addr", macAddr, oldMac); errOld == nil && oldMac.Id != mi.Id && !oldMac.Keep && !oldMac.KeepUsername {
						_ = dbdata.Del(oldMac)
					}
					mi.MacAddr = macAddr
				}
				_ = dbdata.Set(&mi)
				ipActive[ipStr] = true
				return ip
			}
		}
		base.Error(username, macAddr, "Reserved username IP is active or does not match CIDR")
		return nil
	}

	// Obtaining the client's macAddr (standard MAC and Keep binding)
	if uniqueMac {
		mi := &dbdata.IpMap{}
		err = dbdata.One("mac_addr", macAddr, mi)
		if err != nil {
			if dbdata.CheckErrNotFound(err) {
				return loopIp(username, macAddr, uniqueMac)
			}
			base.Error(err)
			return nil
		}

		base.Trace("uniqueMac:", username, mi)
		ipStr := mi.IpAddr
		ip := net.ParseIP(ipStr)
		_, ok := ipActive[ipStr]
		if !ok && ipInPool(ip) {
			mi.Username = username
			mi.LastLogin = tNow
			mi.UniqueMac = uniqueMac
			_ = dbdata.Set(mi)
			ipActive[ipStr] = true
			return ip
		}

		// IP reservation (MAC)
		if mi.Keep || mi.KeepUsername {
			base.Error(username, macAddr, ipStr, "Reserved ip does not match CIDR or is active")
			return nil
		}

		mi = &dbdata.IpMap{MacAddr: macAddr}
		_ = dbdata.Del(mi)
		return loopIp(username, macAddr, uniqueMac)
	}

	// 3. If you don't have a Mac
	ipMaps := []dbdata.IpMap{}
	err = dbdata.FindWhere(&ipMaps, 30, 1, "username=?", username)
	if err != nil {
		if dbdata.CheckErrNotFound(err) {
			return loopIp(username, macAddr, uniqueMac)
		}
		base.Error(err)
		return nil
	}

	for _, mi := range ipMaps {
		ipStr := mi.IpAddr
		ip := net.ParseIP(ipStr)

		if _, ok := ipActive[ipStr]; ok {
			continue
		}
		if mi.Keep || mi.KeepUsername {
			continue
		}
		if mi.UniqueMac {
			continue
		}

		if ipInPool(ip) {
			mi.Username = username
			mi.LastLogin = tNow
			mi.MacAddr = macAddr
			mi.UniqueMac = uniqueMac
			_ = dbdata.Set(&mi)
			ipActive[ipStr] = true
			return ip
		}
	}

	return loopIp(username, macAddr, uniqueMac)
}

var (
	// Record loop points
	loopCurIp uint32
	loopFarIp *dbdata.IpMap
)

func loopIp(username, macAddr string, uniqueMac bool) net.IP {
	var (
		i  uint32
		ip net.IP
	)

	// Reassignment
	loopFarIp = &dbdata.IpMap{LastLogin: time.Now()}

	i, ip = loopLong(loopCurIp, IpPool.IpLongMax, username, macAddr, uniqueMac)
	if ip != nil {
		loopCurIp = i
		return ip
	}

	i, ip = loopLong(IpPool.IpLongMin, loopCurIp, username, macAddr, uniqueMac)
	if ip != nil {
		loopCurIp = i
		return ip
	}

	// After IP allocation, start from the beginning
	loopCurIp = IpPool.IpLongMin

	if loopFarIp.Id > 0 {
		// Use the earliest logged in IP
		ipStr := loopFarIp.IpAddr
		ip = net.ParseIP(ipStr)
		mi := &dbdata.IpMap{IpAddr: ipStr, MacAddr: macAddr, UniqueMac: uniqueMac, Username: username, LastLogin: time.Now()}
		// Write back db data
		_ = dbdata.Set(mi)
		ipActive[ipStr] = true

		return ip
	}

	// All online, no data available
	base.Warn("no ip available, please see ip_map table row", username, macAddr)
	return nil
}

func loopLong(start, end uint32, username, macAddr string, uniqueMac bool) (uint32, net.IP) {
	var (
		err       error
		tNow      = time.Now()
		leaseTime = time.Now().Add(-1 * time.Duration(base.Cfg.IpLease) * time.Second)
	)

	// Global traversal of expired and unreserved IPs
	for i := start; i <= end; i++ {
		ip := utils.Long2ip(i)
		ipStr := ip.String()

		// Skip active connections
		if _, ok := ipActive[ipStr]; ok {
			continue
		}

		mi := &dbdata.IpMap{}
		err = dbdata.One("ip_addr", ipStr, mi)
		if err != nil {
			// no data has been found
			if dbdata.CheckErrNotFound(err) {
				// This ip is not in use
				mi = &dbdata.IpMap{IpAddr: ipStr, MacAddr: macAddr, UniqueMac: uniqueMac, Username: username, LastLogin: tNow}
				_ = dbdata.Add(mi)
				ipActive[ipStr] = true
				return i, ip
			}
			// Query error report
			base.Error(err)
			return 0, nil
		}

		// Query the used IP
		// Skip reserved ip (by MAC or Username)
		if mi.Keep || mi.KeepUsername {
			continue
		}
		// Determine the lease term
		if mi.LastLogin.Before(leaseTime) {
			// There is a record indicating that the lease period has expired and can be used directly.
			mi.Username = username
			mi.LastLogin = tNow
			mi.MacAddr = macAddr
			mi.UniqueMac = uniqueMac
			// Write back db data
			_ = dbdata.Set(mi)
			ipActive[ipStr] = true
			return i, ip
		}
		// Other situations determine the earliest landing
		if mi.LastLogin.Before(loopFarIp.LastLogin) {
			loopFarIp = mi
		}
	}

	return 0, nil
}

// Recycle ip
func ReleaseIp(ip net.IP, macAddr string) {
	ipPoolMux.Lock()
	defer ipPoolMux.Unlock()

	delete(ipActive, ip.String())

	mi := &dbdata.IpMap{}
	err := dbdata.One("ip_addr", ip.String(), mi)
	if err == nil {
		mi.LastLogin = time.Now()
		_ = dbdata.Set(mi)
	}
}
