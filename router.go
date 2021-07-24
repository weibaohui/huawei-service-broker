package main

import (
	"github.com/astaxie/beego"
)

func InitRoutes() {
	var ctr = Controller{}
	//实现Broker要求的几个RestAPI，有些是可选的，具体看《服务发布规范》
	beego.Router("/v2/catalog", &ctr, "get:GetServiceCatalog")
	beego.Router("/v2/service_instances/:instance_id", &ctr, "put:CreateInstance")
	beego.Router("/v2/service_instances/:instance_id", &ctr, "delete:DeleteInstance")
	beego.Router("/v2/service_instances/:instance_id", &ctr, "patch:UpdateInstance")
	beego.Router("/v2/service_instances/:instance_id/service_bindings/:binding_id", &ctr, "put:CreateBinding")
	beego.Router("/v2/service_instances/:instance_id/service_bindings/:binding_id", &ctr, "delete:DeleteBinding")
	beego.Router("/v2/service_instances/:instance_id/last_operation", &ctr, "get:LastOpertaion")
	beego.Router("/v2/service_instances/:instance_id/status", &ctr, "get:GetInstanceStatus")
	//测试自定义订购页面，自定义实例更新页面
	beego.Router("/v2/provision", &ctr, "get:ProvisionWeb")
	beego.Router("/v2/update", &ctr, "get:UpdateWeb")
}
