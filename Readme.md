Web Interface to ElektrikGreen Firefly.

Install MariaDB on the firefly.

sudo apt install mariadb-server

Download the script file 'bin/database build script.sql' from this repository to the firefly and execute the command...

mariadb </projects/FireflyWeb/bin/'database build script'

This script should create a new database called firefly and a table inside it called logging. The logging table has colums
for all the data that will be collected from the firefly. The logged column is the primary key and is defaulted ot the date/time of the server.
A user called FireflyService is created with the ability to write data into the table and read data from the table. This is the user that the Web
interface logs into the datase as.

download the FFDownload.sh script.

Download the web folder from this repository and place it in the /esm folder on the firefly as a child folder. This is where any html or script files
will be served from. Any file placed in this folder will be accessible to a user connecting tot he firefly from any Web browser on port 20080.

Create a directory to contain the github cloned project and clone it to that directory.

mkdir /projects

cd /projects

git clone https://github.com/IanAber/FireflyWeb

