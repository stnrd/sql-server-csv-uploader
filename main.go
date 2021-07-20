package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/joho/sqltocsv"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type ClientConfig struct {
	FtpHost     string `json:"ftp_host"`
	FtpPort     string `json:"ftp_port"`
	FtpUser     string `json:"ftp_user"`
	FtpPass     string `json:"ftp_pass"`
	FtpFolder   string `json:"ftp_folder"`
	Localpath   string `json:"localpath"`
	DbHost      string `json:"db_host"`
	DbPort      string `json:"db_port"`
	DbUser      string `json:"db_user"`
	DbPass      string `json:"db_pass"`
	DbName      string `json:"db_name"`
	DbTimeOut   string `json:"db_timeout"`
	UploadFiles bool   `json:"upload_files"`
	DeleteFiles bool   `json:"delete_files"`
	ZipFiles    bool   `json:"zip_files"`
	Files       []File `json:"files"`
}

type File struct {
	Filename string `json:"filename"`
	Query    string `json:"query"`
}

func main() {
	// Get the program arguments
	configJSON := flag.String("conf", "./Config.json", "Point to the client tool JSON config client")
	flag.Parse()

	// Dereference *pathValue
	ConfJSON := *configJSON

	// intialize
	fmt.Println("Loading config from JSON")

	// Import Config JSON
	cc := ClientConfig{}
	err := cc.loadClientConfig(ConfJSON)
	if err != nil {
		fmt.Printf("Error importing json config, check path (%s)\n", ConfJSON)
		log.Fatal(err)
	}

	// Get passwords from os enviroment variables if empty
	if cc.DbPass == "" {
		if !(os.Getenv("DB_PASS") == "") {
			cc.DbPass = os.Getenv("DB_PASS")
		} else {
			log.Fatal("Missing database password")
		}
	}

	if cc.FtpPass == "" {
		if !(os.Getenv("FTP_PASS") == "") {
			cc.DbPass = os.Getenv("FTP_PASS")
		} else {
			log.Fatal("Missing ftp password")
		}
	}

	// Create connection string
	host, port := parseMSSQLHostPort(cc.DbHost)
	connStr := fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
		host, port, cc.DbName, cc.DbUser, cc.DbPass)

	// Connect to database
	db, err := connectToDatabase("sqlserver", connStr)
	if err != nil {
		fmt.Printf("Error connecting to database")
	}
	defer db.Close()

	// Loop through query's and stores results as csv
	for _, file := range cc.Files {
		fmt.Printf("%s Loading Started\n", file.Filename)

		// Create filename
		fileName := createFileName(file.Filename)
		filePath := cc.Localpath + "/" + fileName

		// Get rows from sql
		rows, err := db.Query(file.Query)
		if err != nil {
			log.Fatal(err)
		}

		// Write rows to CSV file
		err = sqltocsv.WriteFile(filePath, rows)
		if err != nil {
			log.Fatal(err)
		}

		// Zip files
		if cc.ZipFiles {
			// Perform zip operation
			err = ZipFile(fileName, filePath)
			if err != nil {
				log.Fatal(err)
			}

			// Set filenames to zip archvie
			fileName = strings.Replace(fileName, ".csv", ".zip", -1)
			filePath = strings.Replace(filePath, ".csv", ".zip", -1)
		}

		//Upload file to remote directory
		if cc.UploadFiles {
			err = uploadFile(cc, fileName, filePath) // Check if client config really needs passing through
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("File %s uploaded \n", fileName)
		}

		//Delete existing file
		if cc.DeleteFiles {
			err = os.Remove(filePath)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func connectToDatabase(dbType string, connString string) (*sql.DB, error) {
	db, err := sql.Open(dbType, connString)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func createFileName(source string) string {
	t := time.Now()
	var buffer bytes.Buffer
	buffer.WriteString(source)
	buffer.WriteString("_")
	buffer.WriteString(t.Format("2006-01-02"))
	buffer.WriteString(".csv")
	return buffer.String()
}

// Getting information from JSON config
func (c *ClientConfig) loadClientConfig(jsonConfig string) error {

	file, err := ioutil.ReadFile(jsonConfig)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, c)
	if err != nil {
		return err
	}

	return nil
}

// ZipFile receives a filename and file path and creates a zip archive based on the same name
func ZipFile(fileName, filePath string) error {
	// target zip file
	zipFile, err := os.Create(strings.Replace(fileName, ".csv", ".zip", -1))
	if err != nil {
		return err
	}
	defer zipFile.Close()

	// Create Zip archive
	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	// Add file metadata to the zip archive
	writer, err := archive.Create(fileName)
	if err != nil {
		log.Fatal(err)
	}

	// Open csv file for copying
	csvFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	// Transfer the bytes form the csvfile to the zip archive
	_, err = io.Copy(writer, csvFile)
	if err != nil {
		return err
	}

	// Close csv file
	csvFile.Close()

	// Delete CSV files
	err = os.Remove(filePath)
	if err != nil {
		return err
	}

	return nil
}

// Upload data to SFTP server over a secure connection
func uploadFile(config ClientConfig, srcFileName string, srcFilePath string) error {
	// Get content csv to bytes array, so it can be transfered over SFTP
	content, err := ioutil.ReadFile(srcFilePath)
	if err != nil {
		log.Fatal(err)
	}
	destFile := config.FtpFolder + "/" + srcFileName
	addr := config.FtpHost + ":" + config.FtpPort
	sshConfig := &ssh.ClientConfig{
		User: config.FtpUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.FtpPass),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		panic("Failed to dial: " + err.Error())
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		panic("Failed to create client: " + err.Error())
	}

	// Put file to remote FTP server
	file, err := client.Create(destFile)
	if err != nil {
		log.Fatal(err)
	}

	// Transfer content to file
	transfer, err := file.Write(content)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Bytes transfered: %d \n", transfer)

	// Close connection
	defer file.Close()
	defer client.Close()
	defer conn.Close()
	cwd, err := client.Getwd()
	println("Current working directory:", cwd)

	return nil
}

func parseMSSQLHostPort(info string) (string, string) {
	host, port := "127.0.0.1", "1433"
	if strings.Contains(info, ":") {
		host = strings.Split(info, ":")[0]
		port = strings.Split(info, ":")[1]
	} else if strings.Contains(info, ",") {
		host = strings.Split(info, ",")[0]
		port = strings.TrimSpace(strings.Split(info, ",")[1])
	} else if len(info) > 0 {
		host = info
	}
	return host, port
}
