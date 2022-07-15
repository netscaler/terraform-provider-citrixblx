## Terraform provider for Citrix BLX
Citrix has developed a custom Terraform provider for automating [Citrix BLX](https://docs.citrix.com/en-us/citrix-adc-blx/current-release.html) deployments and configurations. Using [Terraform](https://www.terraform.io), you can [deploying and configure a Citrix ADC BLX](https://www.youtube.com/watch?v=3hNWfRKidNI). 

Learn more about Citrix ADC BLX Automation [here](https://github.com/citrix/terraform-provider-citrixadc) 

**Important note: The provider is not able to read remote state of the deployed BLX.**

## Table of contents

* [Why Terraform for Citrix ADC BLX ?](#why-terraform-for-citrix-adc-)
* [Navigating Repository](#navigating-the-repository)
* [Installating Terraform and Citrix BLX Provider](#installation)
* [Get Started on using terraform to deploy Citrix BLX](#get-started-on-configuring-adc-through-terraform)
* Usage Guidelines
  - [Understanding Provider Configuration](#understanding-provider-configuration)
  - [Understanding Resource Configuration](#resource-configuration)
  - [Building your own provider](#building)


## Why Terraform for Citrix BLX ?

[Terraform](https://www.terraform.io/) is an open-source infrastructure as code software tool that provides a consistent CLI workflow to manage hundreds of cloud services.Terraform codifies cloud APIs into declarative configuration files.
Terraform can be used to **_deploy_** and **_configure_** ADC BLX. Configuring Citrix ADC BLX through Terraform provides multiple benefits.

1. ADC BLX resource file in Terraform is human friendly and easy to understand.
2. Abstract away the complexity associated with Citrix ADC BLX deployments.


## Requirement

* [hashicorp/terraform](https://github.com/hashicorp/terraform)

To run the terraform-citrixadc-provider you need to have the hashicorp terraform executable
installed in your system.

You can download it from this [page](https://releases.hashicorp.com/terraform/).

## Navigating the repository

1. _citrixblx folder_ - Contains the citrixblx resource file and modules leveraged by Terraform.
2. _examples folder_ - Contain the examples for users to deploy BLX.

## Installation

### **Step 1. Installing Terraform CLI:**
First step is to install Terraform CLI. Refer the https://learn.hashicorp.com/tutorials/terraform/install-cli for installing Terraform CLI. 

### **Step 2. Installing Citrix BLX Provider:**
Terraform provider for Citrix ADC is not available through terrform.registry.io as of now. Hence users have to install the provider manually.

#### **Follow below steps to install citrix adc provider for Terraform CLI version < 13.0**
1. Download the citrix adc terraform binary in your local machine where you have terraform installed from the [Releases section of the github repo](https://github.com/citrix/terraform-provider-citrixblx/releases).Untar the files and you can find the binary file terraform-provider-ctitrixblx.

2. Edit .terraformrc for the base directory of plugins:
```
plugin_cache_dir = "/home/user/.terraform.d/plugins"
```
3. Copy terrafom-provider-citrixadc binary in appropriate location - `$plugin_cache_dir/<platform>/terraform-provider-citrixblx`.
e.g. `/home/user/.terraform.d/plugins/linux_amd64/terraform-provider-citrixblx`

#### **Follow below steps to install citrix adc provider for Terraform CLI version >13.0**
1. Download the citrix blx terraform binary in your local machine where you have terraform installed from the [Releases section of the github repo](https://github.com/citrix/terraform-provider-citrixadc/releases).Untar the files and you can find the binary file terraform-provider-citrixblx.

2. Create a following directory in your local machine and save the citrix adc terraform binary. e.g. in Ubuntu machine. Note that the directory structure has to be same as below, you can edit the version -0.0.1 to the citrix adc version you downloaded.
```
mkdir -p /home/user/.terraform.d/plugins/registry.terraform.io/citrix/citrixblx/0.0.1/linux_amd64/
```
3. Copy the terraform-provider-citrixadc to the above created folder as shown below
```
cp terraform-provider-citrixblx /home/user/.terraform.d/plugins/registry.terraform.io/citrix/citrixblx/0.0.1/linux_amd64/
```

## Get Started on Configuring ADC through Terraform
_In order to familiarize with citrix blx deployment through terraform, lets get started with basic configuration of setting up a dedicated mode BLX in Terraform.

Before we configure, clone the github repository in your local machine as follows:
```
git clone https://github.com/citrix/terraform-provider-citrixblx/
```
**Step-1** : Now navigate to examples folder as below. Here you can find many ready to use examples for you to get started:
```
cd terraform-provider-citrixblx/examples/
```
Lets configure a simple server in citrix ADC.
```
cd terraform-provider-citrixblx/examples/simple-blx-shared/
```
**Step-2** : Provider.tf contains the details of the target Citrix ADC.Edit the `simple_server/provider.tf` as follows and add details of your target adc.
For **terraform version > 13.0** edit the provider.tf as follows
```
terraform {
    required_providers {
        citrixblx = {
            source = "citrix/citrixblx"
        }
    }
}
provider "citrixblx" {
 }
```
For **terraform version < 13.0**, edit the `provider.tf` as follows
```
provider "citrixblx" {
 }
 ```
**Step-3** : Resources.tf contains the desired state of the resources that you want to manage through terraform. Here we want to create a shared mode blx. Edit the `simple-blx-shared/resources.tf` with your configuration values - host ip address, host username, host password, blx password  as below. 
```
resource "citrixblx_adc" "blx_1" {
        source = "/home/user/blx-rpm.tar.gz"
        host = {
                ipaddress = "10.20.30.40"
                username  = "user"
                password  = "DummyHostPass"
        }
        config = {
                worker_processes = "2"
        }
        password = "DummyPassword"
}
```
**Step-4** : Once the provider.tf and resources.tf is edited and saved with the desired values in the simple-blx-shared folder, you are good to run terraform and configure ADC.Initialize the terraform by running `terraform-init` inside the simple_server folder as follow:
```
terraform-provider-citrixblx/examples/simple-blx-shared$ terraform init
```
You should see following output if terraform was able to successfully find citrix blx provider and initialize it -
```
Initializing the backend...

Initializing provider plugins...
- Reusing previous version of hashicorp/citrixblx from the dependency lock file
- Installing hashicorp/citrixblx v0.0.1...
- Installed hashicorp/citrixblx v0.0.1 (unauthenticated)

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```


## Usage Guidelines

### Understanding Provider Configuration
Provider.tf enables Citrix BLX provider 
```
provider "citrixblx" {
}
```

### Resource Configuration
Resources.tf contains the desired BLX resources which need to be deployed.

All valid options inside the Citrix BLX resource include - 

```
resource "citrixblx_adc" <resource_name> { 
  source = <path-to-blx-tar.gz>
  host = {
    ipaddress         = <host_ipaddress>
    username          = <host_username>
    password          = <host_password>
    port              = <host_ssh_port>
    ssh_hostkey_check = <yes/true when strict hostkey checking must be enabled>
    keyfile           = <key_file_path>
  }

config = {
    ipaddress          = <ip address for BLX>
    interfaces         = <space seperated string of interfaces for BLX>
    worker_processes   = <blx worker process or core mask, eg -{"1" or "-c 0x1"}>
    mgmt_ssh_port      = <mgmt ssh port, shared mode>
    mgmt_http_port     = <mgmt http port, shared mode>
    mgmt_https_port    = <mgmt https port, shared mode>
    total_hugepage_mem = <total hugepage memory to allocate for BLX>
    host_ipaddress     = <host ip address to set if blx_managed_host>
    blx_managed_host   = <1, when host management needs to be dedicated to blx>
    nsdrvd             = <number of nsdrvd process to be enabled>
    cpu_yield          = <yes, when needed to be enabled>
    default_gateway    = <default gateway for the blx>
  }
  
  password = <blx_password to be set, required field>
  
  local_license = [
      <array of paths to local license file> 
  ]
  cli_cmd = [
      <cli_cmd’s to be appended to cli-cmd section of blx.conf>
   ]
   mlx_ofed   = <path_to_mlx_ofed_iso, can be zipped or unzipped>
   mlx_tools  = <path_to_mlx_tools>
}

```

E.g. For creating a shared mode BLX

**`citrixblx_adc`**
```
resource "citrixblx_adc" "blx_1" {
        source = "/home/user/blx-rpm.tar.gz"

        host = {
                ipaddress = "10.20.30.40"
                username  = "user"
                password  = "DummyHostPass"
        }

        config = {
                worker_processes = "2"
        }

        password = "DummyPassword"
}
```

#### Structure
* `resources.tf` describes the actual NetScaler config objects to be created. The attributes of these resources are either hard coded or looked up from input variables in `terraform.tfvars`
* `variables.tf` describes the input variables to the terraform config. These can have defaults
* `provider.tf` is used to initialize the citrixblx provider
* `terraform.tfvars` has the variable inputs specified in `variables.tf`

#### Using
Modify the `terraform.tfvars` to suit your own BLX deployment. Use `terraform plan` and `terraform apply` to configure the NetScaler.

#### Updating your configuration
Modify the set of backend services and use `terraform plan` and `terraform apply` to verify the changes

## Building
### Assumption
* You have (some) experience with Terraform, the different provisioners and providers that come out of the box,
its configuration files, tfstate files, etc.
* You are comfortable with the Go language and its code organization.

1. Install `terraform` from <https://www.terraform.io/downloads.html>
2. Check out this code: `git clone https://<>`
3. Build this code using `make build`
4. Binary can be found at `$GOPATH/bin/terraform-provider-citrixblx`
