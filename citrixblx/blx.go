package citrixblx

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	blxTarballName = "blx.tar.gz"
	blxConfigFile  = "/etc/blx/blx.conf"
	blxLicensePath = "/nsconfig/license"

	distRPM = "rpm"
	distDEB = "deb"
)

type blx struct {
	id             string
	source         string
	host           map[string]string
	config         map[string]string
	mlx            map[string]string
	hostSession    *ssh.Client
	nsSession      *ssh.Client
	cliCmd         []string
	licenseList    []string
	dist           string
	password       string
	managementMode bool
	filePath       map[string]string
}

func getHostInfo(d map[string]interface{}) map[string]string {
	var hostKeyList = []string{
		"ipaddress",
		"username",
		"password",
		"keyfile",
		"port",
		"ssh_hostkey_check",
	}
	var host = make(map[string]string)
	for _, key := range hostKeyList {
		if d[key] == nil {
			host[key] = ""
		} else {
			host[key] = d[key].(string)
		}
	}

	return host
}

func getConfigInfo(d map[string]interface{}) map[string]string {
	var configKeyList = []string{
		"ipaddress",
		"interfaces",
		"mgmt_ssh_port",
		"mgmt_http_port",
		"mgmt_https_port",
		"worker_processes",
		"nsdrvd",
		"cpu_yield",
		"default_gateway",
		"total_hugepage_mem",
		"blx_managed_host",
		"host_ipaddress",
	}
	var config = make(map[string]string)
	for _, key := range configKeyList {
		if d[key] == nil {
			config[key] = ""
		} else {
			config[key] = d[key].(string)
		}
	}
	return config
}

func validateBLX(b blx) error {
	if b.host["ipaddress"] == "" {
		return fmt.Errorf("IP Address not provided for BLX Host")
	}

	if b.host["keyfile"] != "" && b.host["password"] != "" {
		return fmt.Errorf("Both KeyFile and Password provided for BLX Host")
	}

	if net.ParseIP(b.id) == nil {
		return fmt.Errorf("Invalid Mgmt IP getting set for BLX %s, for shared mode = Host IP, for dedicated string expected = IP addr, or CIDR notation", b.id)
	}

	if b.password == "" {
		return fmt.Errorf("Password field must be set for BLX")
	}

	return nil
}

func initBLXVar(b *blx) {
	b.filePath["licenseDir"] = fmt.Sprintf("%s/license", b.filePath["terraformInstallDir"])

	b.filePath["blxInstallPath"] = fmt.Sprintf("%s/blx_install", b.filePath["terraformInstallDir"])

	b.filePath["mlxDir"] = fmt.Sprintf("%s/mellanox", b.filePath["terraformInstallDir"])

	b.filePath["blxStartScript"] = fmt.Sprintf("%s/blx_start.sh", b.filePath["terraformInstallDir"])
	b.filePath["blxStopScript"] = fmt.Sprintf("%s/blx_stop.sh", b.filePath["terraformInstallDir"])

	b.filePath["blxStartLog"] = fmt.Sprintf("%s/blx_start.log", b.filePath["terraformInstallDir"])
	b.filePath["blxStopLog"] = fmt.Sprintf("%s/blx_stop.log", b.filePath["terraformInstallDir"])
}

func printDebugHostInfo(b *blx) {
	execSudoCmdHost(b, "ip -br addr show")
	execSudoCmdHost(b, "ip -o link")
	execSudoCmdHost(b, "ip route show")
}

func initBLXHost(b *blx) error {
	b.filePath = make(map[string]string)
	b.filePath["terraformInstallDir"] = "~/.terraform_blx"
	_, err := execCmdHost(b, fmt.Sprintf("mkdir -p %s", b.filePath["terraformInstallDir"]))
	if err != nil {
		return fmt.Errorf("Terraform install path creation failed on host - %s", b.filePath["terraformInstallDir"])
	}
	out, err := execCmdHost(b, fmt.Sprintf("cd %s ; pwd", b.filePath["terraformInstallDir"]))
	b.filePath["terraformInstallDir"] = strings.TrimSpace(out)
	if err != nil {
		return fmt.Errorf("Host Initialization Failed.Error -\n%v", err)
	}
	initBLXVar(b)
	updateDist(b)
	printDebugHostInfo(b)
	return nil
}

func initMLX(b *blx) error {
	execSudoCmdHost(b, fmt.Sprintf("rm -rf %s; mkdir -p %s", b.filePath["mlxDir"], b.filePath["mlxDir"]))

	if b.mlx["ofed"] != "" {
		err := getFile(b.hostSession, b.mlx["ofed"], b.filePath["mlxDir"])
		if err != nil {
			return err
		}

		if strings.HasSuffix(b.mlx["ofed"], ".gz") {
			_, err := execSudoCmdHost(b, fmt.Sprintf("gunzip %s/%s", b.filePath["mlxDir"], filepath.Base(b.mlx["ofed"])))
			if err != nil {
				return err
			}
			b.mlx["ofed"] = strings.TrimSuffix(b.mlx["ofed"], ".gz")
		}
		execSudoCmdHost(b, "umount -f /mnt/mlnxofedinstall")

		// mount the mellanox OFED iso
		_, err = execSudoCmdHost(b, fmt.Sprintf("mount -o ro,loop %s/%s /mnt", b.filePath["mlxDir"], filepath.Base(b.mlx["ofed"])))
		if err != nil {
			return err
		}

		// install kernel headers
		var instCmd string
		var pkgList []string
		if b.dist == distRPM {
			instCmd = "yum install -y "
			pkgList = []string{
				"\"kernel-devel-uname-r == $(uname -r)\"",
			}
		} else {
			instCmd = "apt install -y "
			pkgList = []string{
				"linux-headers-`uname -r`",
			}
		}
		for _, pkg := range pkgList {
			execCmdHost(b, fmt.Sprintf("%s %s", instCmd, pkg))
		}

		// run the install
		ofedInstallCmd := "/mnt/mlnxofedinstall --add-kernel-support --skip-repo --skip-distro-check --skip-unsupported-devices-check"
		out, err := execSudoCmdHost(b, ofedInstallCmd)
		if err != nil {
			log.Printf("[WARN]  citrixblx-provider: output = %s", out)
			if strings.Contains(out, "Current operation system is not supported") {
				id, err1 := execSudoCmdHost(b, "source /etc/os-release && echo \\$ID")
				version, err2 := execSudoCmdHost(b, "source /etc/os-release && echo \\$VERSION_ID")
				if err1 != nil || err2 != nil || len(strings.TrimSpace(id)) == 0 || len(strings.TrimSpace(version)) == 0 {
					log.Printf("[ERROR] Unable to detect OS distro for OFED Installation. Output of OFED Installation =\r\n%v", out)
					return err
				}
				_, err = execSudoCmdHost(b, fmt.Sprintf("%s --distro %s%s", ofedInstallCmd, strings.TrimSpace(id), strings.TrimSpace(version)))
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	// restart driver
	_, err := execSudoCmdHost(b, "/etc/init.d/openibd restart")
	if err != nil {
		return err
	}

	if b.mlx["tools"] != "" {
		// copy tools
		err = getFile(b.hostSession, b.mlx["tools"], fmt.Sprintf("%s/tools", b.filePath["mlxDir"]))
		if err != nil {
			return err
		}

		// install tools
		_, err = execSudoCmdHost(b, fmt.Sprintf("cd %s/tools ; tar xzf * ; cd \\\"\\$(ls -rlth | grep ^d | awk '{print \\$9}')\\\" ; ./install.sh", b.filePath["mlxDir"]))
		if err != nil {
			return err
		}

		_, err = execSudoCmdHost(b, "mst start")
		if err != nil {
			return err
		}
		execSudoCmdHost(b, "mst status -v")
	}

	return nil
}

func setupBLX(b *blx) error {
	err := installBLX(b)
	if err != nil {
		return err
	}
	log.Printf("[INFO]  citrixblx-provider: Installation of BLX for BLX %s SUCCESS", b.id)

	// BLX shouldn't start on host reboot
	execSudoCmdHost(b, "systemctl disable blx")

	enableCoreDumps(b)

	enableRsyslog(b)

	// clear previous present config
	execSudoCmdHost(b, "rm -f /nsconfig/ns.conf*")
	execSudoCmdHost(b, "rm -f /configdb/nscfg.db")
	execSudoCmdHost(b, "rm -f /var/clusterd/*")

	// install mlx ofed and mst tools
	if b.mlx["ofed"] != "" || b.mlx["tools"] != "" {
		err = initMLX(b)
		if err != nil {
			return err
		}
	}

	err = initBLX(b)
	if err != nil {
		return err
	}
	log.Printf("[INFO]  citrixblx-provider: Initialization of BLX for BLX %s SUCCESS", b.id)

	return nil
}

func destroyBLX(b *blx) error {
	err := stopBLX(b)
	if err != nil {
		return err
	}
	log.Printf("[INFO]  citrixblx-provider: Stopping of BLX %s SUCCESS", b.id)

	//	err = uninstallBLX(b)
	//	if err != nil {
	//		return err
	//	}
	//	log.Printf("[INFO] citrixblx-provider: Uninstallation of BLX % SUCCESS", b.id)
	return nil
}

func updateDist(b *blx) error {
	distMap := map[string]string{
		"yum":     distRPM,
		"apt-get": distDEB,
	}
	for cmd, dist := range distMap {
		out, err := execCmdHost(b, fmt.Sprintf("which %s > /dev/null ; echo $?", cmd))
		if err != nil {
			log.Printf("[ERROR]  citrixblx-provider: Error occured while getting distribution")
			return fmt.Errorf("Failed to get distribution.\r\n%v", err)
		}
		if strings.Contains(out, "0") {
			b.dist = dist
			return nil
		}
	}

	err := fmt.Errorf("Unknown OS distribution")
	log.Printf("[ERROR]  citrixblx-provider:", err)
	return err
}

func uninstallBLX(b *blx) error {
	var err error
	stopBLX(b)
	if b.dist == distRPM {
		execSudoCmdHost(b, "yum remove -y blx")
	} else {
		execSudoCmdHost(b, "apt-get -y purge blx")
	}

	out, err := execSudoCmdHost(b, "systemctl status blx >/dev/null ; echo \\$\\?")
	if err != nil {
		log.Printf("[ERROR]  citrixblx-provider: systemctl check after un-installation failed")
		return fmt.Errorf("Error occurred while un-installing blx.\r\n%v", err)
	}
	if strings.Contains(out, "0") {
		err = fmt.Errorf("BLX package exists after un-installation")
		log.Printf("[ERROR]  citrixblx-provider:", err)
		return err
	}

	// Clear install directory
	_, err = execSudoCmdHost(b, fmt.Sprintf("rm -rf %s/*", b.filePath["terraformInstallDir"]))
	if err != nil {
		return fmt.Errorf("Error occurred while un-installing blx.\r\n%v", err)
	}
	return nil
}

func installEPEL(b *blx) error {
	execCmdHost(b, "yum search epel-release")
	out, err := execSudoCmdHost(b, "yum search epel-release | awk '{print \\$1}' | grep epel-release")
	if err != nil {
		return err
	}

	flag := false
	pkgArr := strings.Split(out, "\n")
	if len(pkgArr) == 0 {
		return fmt.Errorf("Package epel-release not found.\n")
	}
	for _, p := range pkgArr {
		pkg := strings.TrimSpace(p)
		if len(pkg) == 0 {
			continue
		}
		_, err := execSudoCmdHost(b, fmt.Sprintf("yum install -y %s", pkg))
		if err == nil {
			flag = true
		}
	}
	if flag {
		return nil
	}
	return fmt.Errorf("Unable to successfully install any epel-release package. Package List=\n%s", out)
}

func installBLX(b *blx) error {
	err := stopBLX(b)
	if err != nil {
		return err
	}

	execSudoCmdHost(b, fmt.Sprintf("rm -rf %s/*", b.filePath["blxInstallPath"]))
	err = getFile(b.hostSession, b.source, b.filePath["blxInstallPath"])
	if err != nil {
		return fmt.Errorf("Unable to get BLX Install Packages from source for BLX, %s.\r\n%v", b.id, err)
	}
	log.Printf("[INFO]  citrixblx-provider: Copy of BLX packages for BLX %s SUCCESS", b.id)

	var cmd string
	if b.dist == distRPM {
		err := installEPEL(b)
		if err != nil {
			log.Printf("[ERROR] citrixblx-provider: Error encountered while installing dependent package - epel-release")
		}
		cmd = fmt.Sprintf("cd %s ; tar xzf * ; cd \\\"\\$(ls -rlth | grep ^d | awk '{print \\$9}')\\\" ; yum install -y *.rpm", b.filePath["blxInstallPath"])
		_, err = execSudoCmdHost(b, cmd)
		if err != nil {
			cmd = fmt.Sprintf("cd %s ; cd \\\"\\$(ls -rlth | grep ^d | awk '{print \\$9}')\\\" ; yum downgrade -y *.rpm", b.filePath["blxInstallPath"])
			_, err1 := execSudoCmdHost(b, cmd)
			cmd = fmt.Sprintf("cd %s ; cd \\\"\\$(ls -rlth | grep ^d | awk '{print \\$9}')\\\" ; yum reinstall -y *.rpm", b.filePath["blxInstallPath"])
			_, err2 := execSudoCmdHost(b, cmd)
			if err1 != nil && err2 != nil {
				return fmt.Errorf("Error occurred while installing BLX. Error-\r\n%v", err)
			}
		}
	} else {
		cmd = fmt.Sprintf("cd %s ; tar xzf * ; cd \\\"\\$(ls -rlth | grep ^d | awk '{print \\$9}')\\\" ; apt install -y -o Dpkg::Options::=\\\"--force-confold\\\" --allow-downgrades ./*.deb", b.filePath["blxInstallPath"])
		_, err := execSudoCmdHost(b, cmd)
		if err != nil {
			return fmt.Errorf("Error occurred while installing BLX. Error-\r\n%v", err)
		}
	}

	// check blx service is present
	_, err = execSudoCmdHost(b, "systemctl list-unit-files | grep -q blx.service")
	if err != nil {
		log.Printf("[ERROR]  citrixblx-provider: systemctl check after installation failed")
		return fmt.Errorf("Error occurred while installing blx.\r\n%v\nBLX Installation Failed", err)
	}
	return nil
}

func enableRsyslog(b *blx) {
	blxRsyslogConfFile := "/etc/rsyslog.d/blx-rsyslog-enable.conf"

	execSudoCmdHost(b, fmt.Sprintf("> %s", blxRsyslogConfFile))
	var configList []string
	if b.dist == distRPM {
		configList = []string{
			"$ModLoad imudp",
			"$UDPServerRun 514",
		}
	} else {
		configList = []string{
			"module(load=\"imudp\")",
			"input(type=\"imudp\" port=\"514\")",
		}
	}
	createFileHost(b, blxRsyslogConfFile, configList)

	execSudoCmdHost(b, "systemctl restart rsyslog")
}

func enableCoreDumps(b *blx) {
	cmdList := []string{
		"mkdir -p /var/core",
		"echo '/var/core/core-%e-sig%s-user%u-group%g-pid%p-time%t' > /proc/sys/kernel/core_pattern",
		"echo '*       hard        core        unlimited\n*       soft        core        unlimited' > /etc/security/limits.d/core.conf",
		"sed -i -e 's/.*DefaultLimitCORE.*/DefaultLimitCORE=infinity/g' /etc/systemd/system.conf",
		"systemctl daemon-reexec",
	}
	for _, cmd := range cmdList {
		execSudoCmdHost(b, cmd)
	}
}

func createStartScript(b *blx) error {
	execSudoCmdHost(b, fmt.Sprintf("rm -f %s", b.filePath["blxStartScript"]))

	startBLXCmd := []string{
		"sleep 2",
		"systemctl restart blx",
	}

	for _, cmd := range startBLXCmd {
		err := appendRemoteFile(b.hostSession, b.filePath["blxStartScript"], cmd)
		if err != nil {
			return fmt.Errorf("Error returned while creating blx start script, Error = %v", err)
		}
	}
	return nil
}

func startBLX(b *blx) error {
	_, err := execSudoCmdHost(b, fmt.Sprintf("nohup bash %s > %s 2>&1 &", b.filePath["blxStartScript"], b.filePath["blxStartLog"]))
	if err != nil {
		fmt.Errorf("Error occurred while starting blx.\r\n%v", err)
	}
	time.Sleep(time.Second * 10)

	err = checkBLXIP(b)
	if err != nil {
		return err
	}
	return nil
}

func restartBLX(b *blx) error {
	log.Printf("[DEBUG]  citrixblx-provider: Stopping BLX for restart")
	err := stopBLX(b)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG]  citrixblx-provider: Starting BLX for restart")
	err = startBLX(b)
	if err != nil {
		return err
	}
	return nil
}

func copyLicense(b *blx) error {
	_, err := execCmdHost(b, fmt.Sprintf("mkdir -p %s", b.filePath["licenseDir"]))
	if err != nil {
		return err
	}
	for _, i := range b.licenseList {
		err = getFile(b.hostSession, i, b.filePath["licenseDir"])
		if err != nil {
			return fmt.Errorf("Error copying license file = %s, Error = %v", i, err)
		}
	}

	_, err = execSudoCmdHost(b, fmt.Sprintf("mv -f %s/* %s", b.filePath["licenseDir"], blxLicensePath))
	if err != nil {
		return err
	}
	_, err = execSudoCmdHost(b, "chown nsroot /nsconfig/license/*")
	if err != nil {
		return err
	}

	execSudoCmdHost(b, fmt.Sprintf("rm -rf %s", b.filePath["licenseDir"]))

	return nil
}

func initBLX(b *blx) error {
	err := stopBLX(b)
	if err != nil {
		return err
	}

	// create blx.conf
	err = createBLXConf(b)
	if err != nil {
		return err
	}

	// copy license files
	if len(b.licenseList) != 0 {
		err = copyLicense(b)
		if err != nil {
			return err
		}
	}

	createStopScript(b)
	createStartScript(b)

	err = startBLX(b)
	if err != nil {
		log.Printf("[ERROR]  citrixblx-provider: Error returned while starting BLX")
		return err
	}

	// pooled licensing
	for _, cmd := range b.cliCmd {
		if strings.Contains(strings.ToLower(cmd), "licenseserver") {
			log.Printf("[DEBUG]  citrixblx-provider: License Server configuration detected, restarting BLX twice %s", b.id)
			err = restartBLX(b)
			if err != nil {
				return err
			}
			err = restartBLX(b)
			if err != nil {
				return err
			}
		}
	}

	// sleep needed for cluster, LA scenario's
	log.Printf("[DEBUG] citrixblx-provider: %s is reachable, sleeping for 90 secs to ensure ports are UP", b.id)
	time.Sleep(90 * time.Second)
	return nil
}

func checkBLXStop(b *blx) error {
	num, err := blxProcessCount(b)
	if err != nil {
		return err
	}
	if num != 0 {
		return fmt.Errorf("Error, BLX Processes still running after stopping BLX")
	}

	return nil
}

func blxProcessCount(b *blx) (int, error) {
	out, err := execSudoCmdHost(b, "ps aux | grep \"/usr/sbin/nsppe\" | grep -v \"grep\" | wc -l")
	if err != nil {
		log.Printf("[ERROR]  citrixblx-provider: Error encountered while checking BLX process, %s", err)
		return 0, fmt.Errorf("Error encountered while checking BLX process on Host.\r\n%v", err)
	}
	var num int
	num, err = strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		log.Printf("[ERROR]  citrixblx-provider: Error encountered while gathering BLX process count, %v, output = %s", err, out)
		return 0, fmt.Errorf("Error encountered while checking BLX process on Host.\r\n%v", err)
	}
	return num, nil
}

func checkBLXProcess(b *blx) error {
	for i := 1; i < 60; i++ {
		num, err := blxProcessCount(b)
		if err != nil {
			return err
		}
		if num == 0 {
			log.Printf("[DEBUG]  citrixblx-provider: Waiting for BLX Processes to come up %d seconds have passed", 2*(i-1))
		} else {
			log.Printf("[DEBUG]  citrixblx-provider: BLX Processes on Host SUCCESS")
			return nil
		}
		time.Sleep(time.Second * 2)
	}

	return fmt.Errorf("BLX processes did not come up Host after 2 mins")
}

func checkBLXIP(b *blx) error {
	mgmtPort := "9022"
	if b.config["mgmt_ssh_port"] != "" {
		mgmtPort = b.config["mgmt_ssh_port"]
	}
	if b.config["ipaddress"] != "" {
		mgmtPort = "22"
	}

	for i := 1; i < 100; i++ {
		_, err := net.DialTimeout("tcp", net.JoinHostPort(b.id, mgmtPort), time.Second*2)
		if err == nil {
			log.Printf("[INFO]  citrixblx-provider: %s:%s is reachable now SUCCESS", b.id, mgmtPort)
			return nil
		}
		time.Sleep(2 * time.Second)
		if i%4 == 0 {
			log.Printf("[WARN]  citrixblx-provider: %s:%s is not reachable, waiting", b.id, mgmtPort)
		}
	}
	log.Printf("[ERROR]  citrixblx-provider: %s:%s not reachable, after waiting for 200 secs", b.id, mgmtPort)
	return fmt.Errorf("BLX not reachable on %s:%s", b.id, mgmtPort)
}

func createStopScript(b *blx) error {
	execSudoCmdHost(b, fmt.Sprintf("rm -f %s;", b.filePath["blxStopScript"]))
	stopBLXCmd := []string{
		"systemctl stop blx",
		"sleep 2",
	}

	for _, cmd := range stopBLXCmd {
		err := appendRemoteFile(b.hostSession, b.filePath["blxStopScript"], cmd)
		if err != nil {
			return fmt.Errorf("Error returned while creating BLX stop script.\r\n%v", err)
		}
	}

	return nil
}

func stopBLX(b *blx) error {
	// stop BLX for destroy
	if b.nsSession != nil {
		runNSShellCmd(b.nsSession, "systemctl stop blx")
		b.nsSession = nil
	}
	time.Sleep(time.Second * 5)

	// re-connect since maybe previously in management mode
	var err error
	b.hostSession, err = hostConnect(b.host)
	if err != nil {
		return fmt.Errorf("Error unable to connect back to host after stopping BLX.\r\n%v", err)
	}
	err = initBLXHost(b)
	if err != nil {
		return fmt.Errorf("Unable to initialize host.\r\n%v", err)
	}

	// re-run the command for terraform install scenario
	execSudoCmdHost(b, "systemctl stop blx")
	_, err = execSudoCmdHost(b, fmt.Sprintf("nohup bash %s > %s 2>&1 &", b.filePath["blxStopScript"], b.filePath["blxStopLog"]))
	if err != nil {
		fmt.Errorf("Error running BLX stop script.\r\n%v", err)
	}
	err = checkBLXStop(b)
	if err != nil {
		return err
	}

	return nil
}

func genBLXConfigBlock(b *blx) []string {
	var cmdList = []string{"blx-system-config", "{"}

	if b.config == nil {
		cmdList = append(cmdList, "}")
		return cmdList
	}

	var blxConfVar = map[string]interface{}{
		"worker-processes":   b.config["worker_processes"],
		"cpu-yield":          b.config["cpu_yield"],
		"ipaddress":          b.config["ipaddress"],
		"interfaces":         b.config["interfaces"],
		"mgmt-http-port":     b.config["mgmt_http_port"],
		"mgmt-https-port":    b.config["mgmt_https_port"],
		"mgmt-ssh-port":      b.config["mgmt_ssh_port"],
		"nsdrvd":             b.config["nsdrvd"],
		"host-ipaddress":     b.config["host_ipaddress"],
		"total-hugepage-mem": b.config["total_hugepage_mem"],
		"blx-managed-host":   b.config["blx_managed_host"],
	}
	for k, v := range blxConfVar {
		if v == "" {
			continue
		}

		cmdList = append(cmdList, fmt.Sprintf("%s: %s", k, v))
	}
	cmdList = append(cmdList, "}")

	return cmdList
}

func genBLXCLICmdBlock(b *blx) []string {
	var cmdList = []string{"cli-cmds", "{"}

	for _, cmd := range b.cliCmd {
		cmdList = append(cmdList, cmd)
	}

	if b.password != "" {
		cmdList = append(cmdList, fmt.Sprintf("set system user nsroot -password %s", b.password))
	}

	cmdList = append(cmdList, "}")

	return cmdList
}

func genBLXRouteBlock(b *blx) []string {
	var cmdList = []string{"static-routes", "{"}

	if b.config["default_gateway"] != "" {
		cmdList = append(cmdList, fmt.Sprintf("default %s", b.config["default_gateway"]))
	}
	cmdList = append(cmdList, "}")

	return cmdList
}

func createBLXConf(b *blx) error {
	_, err := execSudoCmdHost(b, fmt.Sprintf("> %s", blxConfigFile))
	if err != nil {
		log.Printf("[ERROR]  citrixblx-provider: Error encountered while clearing blx.conf, %s", err)
		return fmt.Errorf("Error while creating blx config file.\r\n%v", err)
	}

	execSudoCmdHost(b, fmt.Sprintf("echo \\\"%s\\\" >> %s", "#blx.conf generated by terraform#", blxConfigFile))

	cmdList := genBLXConfigBlock(b)
	cmdList = append(cmdList, genBLXRouteBlock(b)...)
	cmdList = append(cmdList, genBLXCLICmdBlock(b)...)

	err = createFileHost(b, "/etc/blx/blx.conf", cmdList)
	if err != nil {
		return fmt.Errorf("Error creating blx.conf - \n%s", err)
	}

	out, err := execSudoCmdHost(b, "cat /etc/blx/blx.conf | grep -v \\\"\\-password\\\"")
	log.Printf("[INFO]  citrixblx-provider: Printing blx.conf -\n%s", out)
	return nil
}
