curl  -X POST -H 'content-type: application/json' -d '{"entity": "veh", "entityid": "1234", "token": "test.test"}' localhost:8080/cgw/v1/token
curl  -X POST -H 'content-type: application/json' -d '{"entity": "veh", "entityid": "1234", "token": "test.test"}' localhost:8080/cgw/v1/token/validate
curl  -X POST -H 'content-type: application/json' -d '{"entity": "veh", "entityid": "1234", "token": "test.test2"}' localhost:8080/cgw/v1/token/refresh
curl  -X POST -H 'content-type: application/json' -d '{"entity": "VEH", "entityid": "1234", "token": "test.test"}' localhost:8080/cgw/v1/token/validate
curl  -X POST -H 'content-type: application/json' -d '{"entity": "veh", "entityid": "1234", "reasonCode": 152}' localhost:8080/cgw/v1/disconnect