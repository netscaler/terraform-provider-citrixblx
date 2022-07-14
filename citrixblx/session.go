package citrixblx

import (
	"bufio"
	"fmt"
	"github.com/tmc/scp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	sudoPassPreStr = "export HISTFILE=/dev/null PASSWD="
	pathEnvPreStr  = "export PATH=$PATH:/usr/local/sbin:/usr/sbin:/usr/local/bin:/usr/bin"
)

func hostConnect(hostInfo map[string]string) (*ssh.Client, error) {
	ipAddress := hostInfo["ipaddress"]
	sshPort := hostInfo["port"]
	if sshPort == "" {
		sshPort = "22"
	}

	var errSession *ssh.Client
	if !checkIP(ipAddress, sshPort) {
		return errSession, fmt.Errorf("Unable to connect to Host - %s:%s", ipAddress, sshPort)
	}

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if hostInfo["ssh_hostkey_check"] == "yes" || hostInfo["ssh_hostkey_check"] == "true" {
		home, err := os.UserHomeDir()
		if err != nil {
			return errSession, fmt.Errorf("Error getting HOME dir. Error = %v", err)
		}
		hostKeyCallback, err = knownhosts.New(fmt.Sprintf("%s/.ssh/known_hosts", home))
		log.Printf("Host KeyFile = %s", fmt.Sprintf("%s/.ssh/known_hosts", home))
		if err != nil {
			return errSession, fmt.Errorf("Error creating hostkeycallback function %v", err)
		}
	}

	var authFunc []ssh.AuthMethod
	if hostInfo["keyfile"] != "" {
		publicKeyAuth, err := publicKeyAuthFunc(hostInfo["keyfile"])
		if err != nil {
			return errSession, fmt.Errorf("Error using keyfile. Error = %v", err)
		}
		authFunc = []ssh.AuthMethod{publicKeyAuth}

	} else if hostInfo["password"] != "" {
		authFunc = []ssh.AuthMethod{ssh.Password(hostInfo["password"])}

	}

	config := &ssh.ClientConfig{
		User:            hostInfo["username"],
		Timeout:         time.Minute * time.Duration(10),
		HostKeyCallback: hostKeyCallback,
		Auth:            authFunc,
	}

	hostaddress := strings.Join([]string{ipAddress, sshPort}, ":")
	client, err := ssh.Dial("tcp", hostaddress, config)
	if err != nil {
		err = fmt.Errorf("Error connecting to Host - %s:%s, Error = %v", ipAddress, sshPort, err)
	}

	return client, err
}

func nsConnect(b *blx) (*ssh.Client, error) {
	var mgmtPort = "9022"
	if b.config["mgmt_ssh_port"] != "" {
		mgmtPort = b.config["mgmt_ssh_port"]
	}
	if b.config["ipaddress"] != "" {
		mgmtPort = "22"
	}

	var errSession *ssh.Client
	if !checkIP(b.id, mgmtPort) {
		return errSession, fmt.Errorf("Unable to connect to NS - %s:%s", b.id, mgmtPort)
	}

	config := &ssh.ClientConfig{
		User:            "nsroot",
		Timeout:         time.Minute * time.Duration(10),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(b.password)},
	}

	hostaddress := strings.Join([]string{b.id, mgmtPort}, ":")
	client, err := ssh.Dial("tcp", hostaddress, config)
	if err != nil {
		err = fmt.Errorf("Error connecting to NS - %s:%s, Error = %v", b.id, mgmtPort, err)
	}

	return client, err
}

func execSudoCmdHost(b *blx, cmd string) (string, error) {
	cmdPath := fmt.Sprintf("%s/sudo-cmd", b.filePath["terraformInstallDir"])
	_, err := runCmd(b.hostSession, fmt.Sprintf("echo \"%s\" > %s", cmd, cmdPath))
	if err != nil {
		finErr := fmt.Errorf("Error while running command  %s.\n%v", cmd, err)
		return "", finErr
	}

	out, err := runCmd(b.hostSession, fmt.Sprintf("%s'%s' ; echo $PASSWD | sudo -S -k -p \"\" bash %s", sudoPassPreStr, b.host["password"], cmdPath))
	if err != nil {
		err = fmt.Errorf("Error running sudo command - %s.\n%v", cmd, err)
	}
	return string(out), err
}

func execCmdHost(b *blx, cmd string) (string, error) {
	return runCmd(b.hostSession, cmd)
}

func checkIP(ipAddress string, port string) bool {
	for i := 1; i < 100; i++ {
		_, err := net.DialTimeout("tcp", net.JoinHostPort(ipAddress, port), time.Second*1)
		if err == nil {
			time.Sleep(2 * time.Second)
			log.Printf("[INFO]  citrixblx-provider: %s:%s is reachable now SUCCESS", ipAddress, port)
			return true
		}
		time.Sleep(2 * time.Second)
		if i%4 == 0 {
			log.Printf("[WARN]  citrixblx-provider: %s:%s is not reachable, waiting", ipAddress, port)
		}
	}
	log.Printf("[ERROR]  citrixblx-provider: %s:%s not reachable, after waiting for 200 secs", ipAddress, port)
	return false
}

func maskPasswd(cmd string) string {
	if !strings.Contains(cmd, sudoPassPreStr) {
		return cmd
	}
	strSplit := strings.Split(cmd, ";")
	for i, str := range strSplit {
		if strings.Contains(str, sudoPassPreStr) {
			strSplit[i] = "export PASSWD=<PASSWD> "
		}
	}
	return strings.Join(strSplit, ";")
}

func runNSShellCmd(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("Unable to create new session for running command - %s, Error = %v", cmd, err)
	}
	defer session.Close()

	cmd = fmt.Sprintf("shell %s", cmd)
	log.Printf("[DEBUG] citrixblx-provider: Executing command - %s", cmd)
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Printf("[WARN] citrixblx-provider: Error returned while running command - %s", cmd)
		log.Printf("[DEBUG] citrixblx-provider: Printing Error - \n %s", out)
		err = fmt.Errorf("Error running command - %s, Error = %v\n%s", cmd, err, out)
	}
	return string(out), err
}

func runNSCmd(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("Unable to create new session for running command - %s, Error = %v", cmd, err)
	}
	defer session.Close()

	log.Printf("[DEBUG] citrixblx-provider: Executing command - %s", cmd)
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Printf("[WARN] citrixblx-provider: Error returned while running command - %s", cmd)
		log.Printf("[DEBUG] citrixblx-provider: Printing Error - \n %s", out)
		err = fmt.Errorf("Error running command - %s, Error = %v\n%s", cmd, err, out)
	}
	return string(out), err
}

func runCmd(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("Unable to create new session for running command - %s, Error = %v", cmd, err)
	}
	defer session.Close()

	printCmd := maskPasswd(cmd)
	cmd = fmt.Sprintf("%s ; %s", pathEnvPreStr, cmd)
	log.Printf("[DEBUG] citrixblx-provider: Executing command - %s", printCmd)
	out, err := session.CombinedOutput(cmd)
	log.Printf("[DEBUG] citrixblx-provider: Printing Output - \n%s", string(out))

	if err != nil {
		log.Printf("[WARN] citrixblx-provider: Error returned while running command - %s", printCmd)
		log.Printf("[DEBUG] citrixblx-provider: Printing Error - \n %s", out)
		err = fmt.Errorf("Error running command - %s, Error = %v\n%s", printCmd, err, out)
	}

	return string(out), err
}

func copyFile(client *ssh.Client, sourceFilePath string, destFilePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	out, errCmd := runCmd(client, fmt.Sprintf("cd %s > /dev/null ; pwd", destFilePath))
	if errCmd == nil {
		destFilePath = strings.TrimSpace(out)
	}

	log.Printf("[DEBUG] citrixblx-provider: Copying file from %s to %s", sourceFilePath, destFilePath)
	err = scp.CopyPath(sourceFilePath, destFilePath, session)
	if err != nil {
		log.Printf("[ERROR] citrixblx-provider: Failed to copy file from %s, to %s", sourceFilePath, destFilePath)
		err = fmt.Errorf("Error copying file from %s to %s, Error = %v", sourceFilePath, destFilePath, err)
	}
	return err
}

func getFile(client *ssh.Client, source string, dest string) error {
	isURL := func(str string) bool { u, err := url.Parse(str); return err == nil && u.Scheme != "" && u.Host != "" }

	_, err := runCmd(client, fmt.Sprintf("mkdir -p %s", dest))
	if err != nil {
		return fmt.Errorf("Error getting - %s, Error = %v", source, err)
	}

	if isURL(source) {
		cmd := fmt.Sprintf("cd %s ; curl -k -O %s", dest, source)
		_, err := runCmd(client, cmd)
		if err != nil {
			log.Printf("[ERROR]  citrixblx-provider: Failed to download File %s", source)
			return fmt.Errorf("Error getting file - %s.\r\nError = %v", source, err)
		}
		return nil
	}

	_, err = os.Stat(source)
	if err == nil {
		err = copyFile(client, source, dest)
		if err != nil {
			log.Printf("[ERROR]  citrixblx-provider: Failed to copy License File %s", source)
			return fmt.Errorf("Error getting file - %s.\r\nError = %v", source, err)
		}
		return nil
	}

	log.Printf("[ERROR]  citrixblx-provider: File %s is neither a valid URL nor a valid path on the host", source)
	return fmt.Errorf("File %s is neither a valid URL nor a valid path on the host", source)

}

func appendRemoteFile(client *ssh.Client, filePath string, str string) error {
	_, err := runCmd(client, fmt.Sprintf("echo \"%s\" >> %s", str, filePath))
	return err
}

func createFileHost(b *blx, remotePath string, lines []string) error {
	file, err := ioutil.TempFile("/tmp", filepath.Base(remotePath))
	if err != nil {
		return fmt.Errorf("Failed to create temp file in /tmp for %s, Error = %v", remotePath, err)
	}

	tmpFilePath, err := filepath.Abs(file.Name())
	if err != nil {
		return fmt.Errorf("Error creating file %s. Error -\n%s", err)
	}

	err = os.Chmod(tmpFilePath, 0600)

	tmpFileName := filepath.Base(tmpFilePath)
	log.Printf("[DEBUG] citrixblx-provider: Created temp file %s", tmpFilePath)

	datawriter := bufio.NewWriter(file)
	for _, line := range lines {
		_, err = datawriter.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("Error while writing to %s to %s", line, tmpFilePath)
		}
	}
	datawriter.Flush()
	file.Close()

	err = getFile(b.hostSession, tmpFilePath, "/tmp")
	if err != nil {
		return err
	} else {
		os.Remove(file.Name())
	}

	_, err = execSudoCmdHost(b, fmt.Sprintf("mv -f /tmp/%s %s", tmpFileName, remotePath))
	if err != nil {
		return err
	}

	return nil
}

func publicKeyAuthFunc(keyPath string) (ssh.AuthMethod, error) {
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		log.Printf("[ERROR] citrixblx-provider: Unable to read KeyFile - %s", keyPath)
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Printf("[ERROR] citrixblx-provider: SSH key sign failed for KeyFile - %s", keyPath)
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}
