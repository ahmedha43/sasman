@echo off
echo y | pscp -pw mushtaq99 central-server root@167.86.73.203:/tmp/central-server
plink -ssh root@167.86.73.203 -pw mushtaq99 -batch "docker cp /tmp/central-server sasman-central:/app/central-server && docker exec sasman-central chmod +x /app/central-server && docker restart sasman-central && sleep 3 && docker ps --filter name=sasman-central"
