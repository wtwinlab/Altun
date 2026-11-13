
getIP() {
    if [ "$local" = "true" ]; then
        echo 127.0.0.1
    else
	    grep -w $1 ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $2}'
	fi
}

getPrivIP(){
    if [ "$local" = "true" ]; then
        echo 127.0.0.1
    else
	    grep -w $1 ${SCRIPT_DIR}/cluster-instance.info | awk '{ print $3}'
	fi
}

genCert(){
    openssl ecparam -name secp521r1 -genkey -param_enc named_curve -out $1.key
    openssl ecparam -genkey -name prime256v1 -out $1.key
    openssl req -new -SHA384 -key $1.key -nodes -out $1.csr -config openssl.conf
    openssl x509 -req -SHA384 -days 3650 -in $1.csr -CA ca.pem -CAkey ca.key -CAcreateserial -out $1.pem -extfile openssl.conf -extensions req_ext -CAcreateserial
}