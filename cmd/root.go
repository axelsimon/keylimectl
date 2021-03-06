/*
Copyright © 2022 axel simon <axel@redhat.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog"
)

const (
	envPrefix = "KEYLIMECTL"
)

// KeylimeOptions is a struct to hold needed variables.
var (
	Debug   bool
	cfgFile string
	apiVer  string
	Client  http.Client
	tlsDir  string
)

// KeylimeConf is a struct to hold configuration data.
type KeylimeConf struct {
	// Map the keylime.conf INI file's fields to Go variables, remapping the original names. A nested struct is needed because of how viper unmarshals INI.
	// ApiVer doesn't exist in keylime.conf as of 2022-02-07
	ApiVer string

	General struct {
		EnableTLS bool `mapstructure:"enable_TLS"`
	}

	Cloud_agent struct {
		AgentHost         string `mapstructure:"cloudagent_ip"`
		VerifierAgentHost string `mapstructure:"cv_cloudagent"`
		AgentPort         int    `mapstructure:"cloudagent_port"`
	}

	Cloud_verifier struct {
		VerifierHost string `mapstructure:"cloudverifier_ip"`
		VerifierPort int    `mapstructure:"cloudverifier_port"`
	}

	Tenant struct {
		RegistrarHost string `mapstructure:"registrar_ip"`
		RegistrarPort int    `mapstructure:"registrar_port"`
		VerifierHost  string `mapstructure:"cloudverifier_ip"`
		VerifierPort  int    `mapstructure:"cloudverifier_port"`
		TLSDir        string `mapstructure:"tls_dir"`
		CACert        string `mapstructure:"ca_cert"`
		MyCert        string `mapstructure:"my_cert"`
		PrivKey       string `mapstructure:"private_key"`
	}

	Registrar struct {
		RegistrarTLSPort int `mapstructure:"registrar_tls_port"`
	}

	// Webapp not really used. Commenting for now.
	// Webapp struct {
	// 	WebappHost string `mapstructure:"webapp_ip"`
	// 	WebappPort int `mapstructure:"webapp_port"`
	// }

	// All these are used in the original tenant.py
	// Add them to our KeylimeConf struct as needed.
	// uuid_service_generate_locally = None
	// agent_uuid = None

	// K = None
	// V = None
	// U = None
	// auth_tag = None

	// tpm_policy = None
	// vtpm_policy = {}
	// metadata = {}
	// allowlist = {}
	// ima_sign_verification_keys = []
	// revocation_key = ""
	// accept_tpm_hash_algs = []
	// accept_tpm_encryption_algs = []
	// accept_tpm_signing_algs = []
	// mb_refstate = None

	// payload = None

	// tpm_instance = tpm()
}

// Instantiate a variable C of type KeylimeConf. We will unmarshall the .ini config into C.
var C KeylimeConf

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Version: "0.0.2",
	Use:     "keylimectl",
	Short:   "A tool to interact with a Keylime cluster",
	Long: `keylimectl allows a user to interact with a Keylime cluster.
	
It provides acces to operations such as adding an agent, checking the status
of an agent or a verifier, and more.

To use keylimectl you need to have a keylime cluster already running.

Find more information at github.com/axelsimon/keylimectl`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().BoolVar(&Debug, "debug", false, "Switch debug comments on or off")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/etc/keylime.conf", "Config file to use")
	rootCmd.PersistentFlags().StringVar(&apiVer, "api-version", "v1", "Keylime API version to use")
	rootCmd.PersistentFlags().StringVar(&tlsDir, "tls-dir", "/var/lib/keylime/cv_ca/", "Base TLS directory")
	// rootCmd.PersistentFlags().StringVar(&myCert, "client-cert", "", "Client TLS certificate")
	// rootCmd.PersistentFlags().StringVar(&privKey, "private-key", "", "Client TLS private key")
	// rootCmd.PersistentFlags().StringVar(&caCert, "ca-cert", "", "Certificate Authority (CA) root certificate")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("testflag", "t", false, "Help message for testflag")

	// TODO: complete or remove improved version flag
	//rootCmd.SetVersionTemplate('{{with .Name}}{{printf "keylimectl - %s " .}}{{end}}{{printf "Version: %s" .Version}}')
	// Iinitialise mutual TLS configuration
	cobra.OnInitialize(initmTLS)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Initialise an instance of a viper conf
	v := viper.New()

	if cfgFile != "" {
		// Use config file passed from the --config flag.
		// We can't use viper.SetConfigFile directly, as it extrapolates type from extension, and keylime.conf is not .ini
		// viper.SetConfigFile(cfgFile)
		// Retrieve directory and base from cfgFile string and pass them to viper, and directly specify config type as INI.
		flagConfigDir := filepath.Dir(cfgFile)
		flagConfigName := filepath.Base(cfgFile)
		v.AddConfigPath(flagConfigDir)
		v.SetConfigName(flagConfigName)
		v.SetConfigType("ini")

		// Keylimectl doesn't load a configuration from the home dir.
		// Disabling for now.
		//	} else {
		//		// Find home directory.
		//		home, err := os.UserHomeDir()
		//		cobra.CheckErr(err)
		//
		//		// Search config in home directory with name "keylime.conf" (without extension).
		//		viper.SetConfigType("ini")
		//		viper.AddConfigPath(home)
		//		viper.SetConfigName("keylime.conf")

		// Not needed, as the flag has a default.
		// } else {
		// 	// Use default config from /etc/keylime.conf, specifying an ini filetype, since the .conf extension does't let viper know what type of file it is.
		// 	viper.SetConfigType("ini")
		// 	viper.AddConfigPath("/etc/")
		// 	viper.SetConfigName("keylime.conf")
	}
	// Set the environment prefix that Viper should expect
	v.SetEnvPrefix(envPrefix)
	// Read in environment variables that match
	v.AutomaticEnv()

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			fmt.Fprintf(os.Stderr, "Error, default config file not found. %v\n", v.ConfigFileUsed())
		} else {
			// Config file was found but another error was produced
			panic(fmt.Errorf("Fatal error using default config file. %w \n", err))
		}
	} else {
		fmt.Fprintln(os.Stderr, "Using config file:", v.ConfigFileUsed())
	}

	// Make Viper unmarshal the values of the config file into C, a config struct of type KeylimeConf.
	err := v.Unmarshal(&C)
	if err != nil {
		panic(fmt.Errorf("Unable to parse configuration into internal keylimectl struct, %v", err))
	}

	// Bind the current command's flags to viper
	// bindFlags(cmd, v)

	// Print security warning regarding TLS.
	if C.General.EnableTLS {
		fmt.Println("TLS enabled. Good.")
	} else {
		fmt.Println("WARNING: TLS is not enabled.")
	}
	// keylime.conf desn't actually define ApiVer, but we have a default from the --api-version flag and set it up here.
	C.ApiVer = apiVer
	fmt.Println("Using API version:", C.ApiVer)

	if Debug {
		// TO DELETE: debug help
		fmt.Printf("\n\n-----\nDEBUG: Are we getting a KeylimeConf config written?\n\tWhat type?\t%T\n\tWhat value?\t%v\n\n", C, C)
		// fmt.Println("DEBUG: RegistrarHost is (from our conf struct):", C.Tenant.RegistrarHost)
		fmt.Println("DEBUG: RegistrarPort is (from our conf struct):", C.Tenant.RegistrarPort)
		// fmt.Println("DEBUG: RegistrarTLSPort is (from our conf struct):", C.Registrar.RegistrarTLSPort)
		fmt.Println("DEBUG: VerifierPort is (from our conf struct):", C.Cloud_verifier.VerifierPort)
		fmt.Println("DEBUG: CaCert is (from our conf struct):", C.Tenant.CACert)
		fmt.Println("DEBUG: MyCert is (from our conf struct):", C.Tenant.MyCert)
		fmt.Println("DEBUG: end of rootCmd init")
		fmt.Println("-----\n\n")
	}
}

func initmTLS() {
	C.Tenant.TLSDir = tlsDir
	C.Tenant.MyCert = tlsDir + "/" + "client-cert.crt"
	C.Tenant.PrivKey = tlsDir + "/" + "client-private.pem"
	C.Tenant.CACert = tlsDir + "/" + "cacert.crt"

	// keylime.conf can define tls_dir as "default" (which means use cv_ca dir). We need to manage this case where tlsDir == "default" rathen than an actual path.
	// If the --tls-dir flags passes its default ("/var/lib/keylime/cv_va"), use , otherwise set TLSDir in our KeylimeConf struct C to the value passed.
	// if tlsDir == "/var/lib/keylime/cv_ca/" {
	// 	C.Tenant.TLSDir = "default"
	// } else {
	// 	C.Tenant.TLSDir = tlsDir
	// }

	// if C.Tenant.TLSDir == "default" {
	// 	C.Tenant.MyCert = "/var/lib/keylime/cv_ca/client-cert.crt"
	// 	C.Tenant.PrivKey = "/var/lib/keylime/cv_ca/client-private.pem"
	// 	C.Tenant.CACert = "/var/lib/keylime/cv_ca/cacert.crt"
	// } else {
	// 	C.Tenant.MyCert = tlsDir + "/" + "client-cert.crt"
	// 	C.Tenant.PrivKey = tlsDir + "/" + "client-private.pem"
	// 	C.Tenant.CACert = tlsDir + "/" + "cacert.crt"

	// Read key pair to create certificate
	cert, err := tls.LoadX509KeyPair(C.Tenant.MyCert, C.Tenant.PrivKey)
	fmt.Println("Using certificate and private key:", C.Tenant.MyCert, C.Tenant.PrivKey)
	if err != nil {
		log.Fatal(err)
	}
	// Create a CA certificate pool, add CACert to it
	caCert, err := ioutil.ReadFile(C.Tenant.CACert)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Using CA Cert file:", C.Tenant.CACert)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// trying to add the Keylime cert, see if it makes a difference
	// It does not: "certificate signed by unknown authority"
	// klmcrtp := "./certs/keylime-cert.crt"
	// klmCert, err := ioutil.ReadFile(klmcrtp)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("Using public Keylime Cert file:", klmcrtp)
	// caCertPool.AppendCertsFromPEM(klmCert)

	// Create a HTTPS client and provide it with the CA Pool and certificate
	Client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				RootCAs:            caCertPool,
				InsecureSkipVerify: true, //InsecureSkipVerify does not appear to work here
				ServerName:         "keylime",
			},
		},
	}

	// TO DELETE before prod
	if Debug {
		//klog.Infof("DEBUG: mTLS: What does Client look like? %s\n\n", Client)
		klog.Infof("DEBUG: mTLS: What does Client Transport look like? %s\n\n", Client.Transport)
		//klog.Infof("DEBUG: mTLS: What does caCertPool look like? %s\n\n", caCertPool)
	}
}
