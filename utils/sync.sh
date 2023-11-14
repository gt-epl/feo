cd ~/feo
go build
rsync feo clabcl0:~/feo/
cd central_server
go build
rsync central_server clabcl1:~/
