import msgpackrpc
import sys

def new_rpc_client():
    address = msgpackrpc.Address('localhost', 50000)
    client = msgpackrpc.Client(address, unpack_encoding='utf-8')
    return client

r = new_rpc_client()
for i in xrange(10):
    print r.call("LBNS.Req", "service1")

try:
    print r.call("LBNS.Req", "service2")
except :
    print "bad service: got exception:", sys.exc_info()[0]

r.call("LBNS.Set", {"Service" : "service1", "Addr": "64.64.64.64:1234", "Weight" : 100 } )

print r.call("LBNS.Req", "service1")
print r.call("LBNS.List", "service1")
