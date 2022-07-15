resource "citrixblx_adc" "blx_1" {
	source = "/home/user/blx-deb.tar.gz"

	host = {
		ipaddress = "2.2.2.2"
		username  = "user"
		ssh_hostkey_check = "true"
		keyfile = "/home/user/login_keyfile"
	}

	config = {
		worker_processes = "-c 0x3"
		interfaces =  "eth0 eth1"
		default_gateway = "3.3.3.1"
		ipaddress = "3.3.3.3"
	}

	password = "DummyPassword"

       local_license = [                            |
               "/home/user/trial.lic"
       ]
}
