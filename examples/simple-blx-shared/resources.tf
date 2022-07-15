resource "citrixblx_adc" "blx_1" {
	source = "/home/user/blx-rpm.tar.gz"

	host = {
		ipaddress = "2.2.2.2"
		username  = "user"
		password  = "DummyHostPass"
	}

	config = {
		worker_processes = "2"
	}

	password = "DummyPassword"
}
