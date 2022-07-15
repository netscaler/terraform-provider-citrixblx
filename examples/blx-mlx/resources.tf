resource "citrixblx_adc" "blx_1" {
	source = "/home/user/blx-rpm.tar.gz"

	host = {
		ipaddress = "2.2.2.2"
		username  = "user"
		keyfile = "/home/user/login_keyfile"
	}

	config = {
		worker_processes = "-c 0x3"
		interfaces =  "eth0"
		default_gateway = "2.2.2.1"
		ipaddress = "2.2.2.5"
		blx_managed_host = 1
		host_ipaddress = "2.2.2.2"
		total_hugepage_mem = "4G"
	}

	mlx_ofed  = "/home/user/MLNX_OFED_LINUX-ol.iso.gz"
	mlx_tools = "home/user/mft-rpm.tgz"

	password = "DummyPassword"

       cli_cmd = [                            |
               "add ns ip 2.2.2.10 255.255.255.0"
       ]
}
