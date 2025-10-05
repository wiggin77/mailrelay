package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/smtp"
	"os"
	"os/signal"
	"syscall"

	"github.com/jpillora/ipfilter"
	log "github.com/phires/go-guerrilla/log"
)

const (
	DefaultSMTPPort        = 465
	DefaultMaxEmailSize    = 83886080 // 80 MB (80 * 1024 * 1024)
	DefaultLocalListenIP   = "0.0.0.0"
	DefaultLocalListenPort = 2525
	DefaultTimeoutSecs     = 300 // 5 minutes
	MinEmailSizeBytes      = 1024
)

// Logger is the global logger.
var Logger log.Logger

// AllowedSendersFilter holds the global list of allowed sender IPs.
var AllowedSendersFilter = ipfilter.New(ipfilter.Options{})

type mailRelayConfig struct {
	SMTPServer        string   `json:"smtp_server"`
	SMTPPort          int      `json:"smtp_port"`
	SMTPStartTLS      bool     `json:"smtp_starttls"`
	SMTPLoginAuthType bool     `json:"smtp_login_auth_type"`
	SMTPUsername      string   `json:"smtp_username"`
	SMTPPassword      string   `json:"smtp_password"`
	SMTPHelo          string   `json:"smtp_helo"`
	SkipCertVerify    bool     `json:"smtp_skip_cert_verify"`
	MaxEmailSize      int64    `json:"smtp_max_email_size"`
	LocalListenIP     string   `json:"local_listen_ip"`
	LocalListenPort   int      `json:"local_listen_port"`
	AllowedHosts      []string `json:"allowed_hosts"`
	AllowedSenders    string   `json:"allowed_senders"`
	TimeoutSecs       int      `json:"timeout_secs"`
	LogLevel          string   `json:"log_level"`
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	configFile, test, testsender, testrcpt, checkIP, ipToCheck, verbose := parseFlags()

	appConfig, err := loadConfig(configFile)
	if err != nil {
		flag.Usage()
		return fmt.Errorf("loading config: %w", err)
	}

	if err := setupIPFilter(appConfig); err != nil {
		return err
	}

	if err := setupLogger(verbose, appConfig.LogLevel); err != nil {
		return err
	}

	if err := Start(appConfig, verbose); err != nil {
		flag.Usage()
		return fmt.Errorf("starting server: %w", err)
	}

	if test {
		return runTest(testsender, testrcpt, appConfig.LocalListenPort)
	}

	if checkIP {
		return runIPCheck(ipToCheck)
	}

	return waitForSignal()
}

func parseFlags() (string, bool, string, string, bool, string, bool) {
	var configFile string
	var test bool
	var testsender string
	var testrcpt string
	var checkIP bool
	var ipToCheck string
	var verbose bool

	flag.StringVar(&configFile, "config", "/etc/mailrelay.json", "specifies JSON config file")
	flag.BoolVar(&test, "test", false, "sends a test message to SMTP server")
	flag.StringVar(&testsender, "sender", "", "used with 'test' to specify sender email address")
	flag.StringVar(&testrcpt, "rcpt", "", "used with 'test' to specify recipient email address")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.BoolVar(&checkIP, "checkIP", false, "checks a provided IP address to see if it would be allowed")
	flag.StringVar(&ipToCheck, "ip", "", "used with 'checkIP' to specify IP address to test")
	flag.Parse()

	return configFile, test, testsender, testrcpt, checkIP, ipToCheck, verbose
}

func setupIPFilter(appConfig *mailRelayConfig) error {
	if appConfig.AllowedSenders == "*" {
		return nil
	}

	file, err := os.Open(appConfig.AllowedSenders)
	if err != nil {
		return fmt.Errorf("failed opening file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var allowedIPsAndRanges []string

	for scanner.Scan() {
		allowedIPsAndRanges = append(allowedIPsAndRanges, scanner.Text())
	}

	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		AllowedIPs:     allowedIPsAndRanges,
		BlockByDefault: true,
	})

	return nil
}

func setupLogger(verbose bool, logLevel string) error {
	if verbose {
		logLevel = "debug"
	}

	var err error
	Logger, err = log.GetLogger("stdout", logLevel)
	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}
	return nil
}

func runTest(testsender, testrcpt string, port int) error {
	err := sendTest(testsender, testrcpt, port)
	if err != nil {
		return fmt.Errorf("sending test message: %w", err)
	}
	return nil
}

func runIPCheck(ipToCheck string) error {
	if ipToCheck == "" {
		return errors.New("IP address to check is required when `checkIP` flag is used. " +
			"Provide an IP address using the `-ip` flag")
	}

	result := ""
	if !AllowedSendersFilter.Blocked(ipToCheck) {
		result = "NOT "
	}
	fmt.Printf("IP address %s is %sallowed to send email\n", ipToCheck, result)
	return nil
}

func waitForSignal() error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	return nil
}

func loadConfig(path string) (*mailRelayConfig, error) {
	var cfg mailRelayConfig
	configDefaults(&cfg)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	parser := json.NewDecoder(file)
	if err := parser.Decode(&cfg); err != nil {
		return nil, err
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func configDefaults(config *mailRelayConfig) {
	config.SMTPPort = DefaultSMTPPort
	config.SMTPStartTLS = false
	config.SMTPLoginAuthType = false
	config.MaxEmailSize = DefaultMaxEmailSize
	config.SkipCertVerify = false
	config.LocalListenIP = DefaultLocalListenIP
	config.LocalListenPort = DefaultLocalListenPort
	config.AllowedHosts = []string{"*"}
	config.AllowedSenders = "*"
	config.TimeoutSecs = DefaultTimeoutSecs
	config.LogLevel = "info"
}

// validateConfig validates the configuration values.
func validateConfig(config *mailRelayConfig) error {
	if config.SMTPServer == "" {
		return errors.New("smtp_server is required")
	}

	if config.SMTPPort < 1 || config.SMTPPort > 65535 {
		return errors.New("smtp_port must be between 1 and 65535")
	}

	if config.LocalListenPort < 1 || config.LocalListenPort > 65535 {
		return errors.New("local_listen_port must be between 1 and 65535")
	}

	if config.MaxEmailSize < MinEmailSizeBytes {
		return errors.New("smtp_max_email_size must be at least 1024 bytes")
	}

	if config.TimeoutSecs < 1 || config.TimeoutSecs > 3600 {
		return errors.New("timeout_secs must be between 1 and 3600 seconds")
	}

	return nil
}

// sendTest sends a test message to the SMTP server specified in mailrelay.json.
func sendTest(sender string, rcpt string, port int) error {
	conn, err := smtp.Dial(fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}

	if err := conn.Mail(sender); err != nil {
		return err
	}
	if err := conn.Rcpt(rcpt); err != nil {
		return err
	}

	if err := writeBody(conn, sender); err != nil {
		return err
	}
	return conn.Quit()
}

func writeBody(conn *smtp.Client, sender string) error {
	wc, err := conn.Data()
	if err != nil {
		return err
	}
	defer wc.Close()
	_, err = fmt.Fprintf(wc, "From: %s\nSubject: Test message\n\nThis is a test email from mailrelay.\n", sender)
	return err
}
