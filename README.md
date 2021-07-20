# SQL Server CSV uploader

This Golang script is extracting data from a SQL Server and stores this data as a CSV file and uploads this file to a sFTP server. This script is able to zip the files and send them compressed and delete them after they have been sent. 

The database connection works with SQL authentication.

Because this script is written in go, this can be compiled to native binaries for windows, linux and other operating systems. 

## Usage 

via command / bash prompt
./sql-server-csv-uploader -conf "./Config.json"

### Go Build
Build binaries for different operating systems 

env GOOS=linux GOARCH=amd64 go build 
env GOOS=linux GOARCH=386 go build 

env GOOS=windows GOARCH=amd64 go build 
env GOOS=windows GOARCH=386 go build 

env GOOS=freebsd GOARCH=amd64 go build 
env GOOS=freebsd GOARCH=386 go build 


## Config JSON 

This script requires a config json that contains all the needed parameters. 

If you don't want passwords via a json file, this script is also able to load them via SYSTEM ENVIROMENT VARIABLE. 

for the FTP password: FTP_PASS
for the Database password: DB_PASS

```
{
    "ftp_host":"", // FTP Host
    "ftp_port":"", // FTP Port 22 for sFTP
    "ftp_user":"", // FTP User
    "ftp_pass":"", // FTP Password 
    "ftp_folder":"", // FTP folder fro example /in 
    "localpath":"", // local path on the server where the script runs 
    "db_host":"", // database host 
    "db_port":"", // database port only requires when not default
    "db_user":"", // SQL User
    "db_pass":"", // SQL password
    "db_name":"", // database name
    "upload_files":true, // Boolean upload the file or not
    "delete_files": true, // Delete files after sending them 
    "zip_files": true, // zip the downloaded CSV files
    "files" : [ // array of files that you want to extract
        {
            "filename":"Inventory", // File name without csv or zip 
            "query":"select top 1000 * from dbo.inventory" // sql query
        },
        {
            "filename":"Sales",
            "query":"select top 1000 * from dbo.sales"
        }
    ]
}
```