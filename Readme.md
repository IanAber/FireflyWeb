Web Interface to ElektrikGreen Firefly.

Install MariaDB on the firefly.

sudo apt install mariadb-server

Create a directory to contain the github cloned project and clone it to that directory.

mkdir /projects

cd /projects

git clone https://github.com/IanAber/FireflyWeb

Create the database and use account.
This script should create a new database called firefly and a table inside it called logging. The logging table has colums
for all the data that will be collected from the firefly. The logged column is the primary key and is defaulted ot the date/time of the server.
A user called FireflyService is created with the ability to write data into the table and read data from the table. This is the user that the Web
interface logs into the datase as.

mariadb </projects/FireflyWeb/bin/'database build script'

Copy the files to the esm directory

sudo cp /projects/FireflyWeb/bin/linux/FireflyWeb /esm
sudo cp -r /projects/FireflyWeb/web /esm

Execute the service

cd /esm
sudo ./FireflyWeb


You should now be able to connect to the server from a web browser by navigating to...

http://<firefly NUC>:20080

If this all works you can run the service by executing, from the esm directory, the command...

nohup ./FireflyWeb &
