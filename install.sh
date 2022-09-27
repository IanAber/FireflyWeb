systemctl stop FuelCellStartupControl
systemctl stop FireflyWeb
cp bin/linux/FireflyWeb /usr/bin
cp -r web/* /Firefly/web
systemctl start FireflyWeb
systemctl start FuelCellStartupControl
