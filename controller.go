package main

import (
	"common"
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego"
	"io/ioutil"
	"net/http"
	"service-broker/aos"
)

type Controller struct {
	beego.Controller
}

//查询 catalog
func (this *Controller) GetServiceCatalog() {
	//因为Broker是服务框架启动的，所以这里仅作为健康检查的API，这里200OK，就认为Broker启动成功
	this.Output(http.StatusOK, "broker status ok")
}

//新建 service_instances
func (this *Controller) CreateInstance() {
	//调用AOS的API，启动实例
	token := this.Ctx.Input.Header("X-Auth-Token")
	instanceId := this.Ctx.Input.Param(":instance_id")
	var req CreateInstReq
	//解析请求
	err := json.Unmarshal([]byte(this.Ctx.Input.RequestBody), &req)
	if err != nil {
		beego.Warn("Unmarshal create request body fail, err:", err)
		common.OutputErrorWithCode(this.Ctx, "Unmarshal request body fail", http.StatusBadRequest)
		return
	}
	stackName := aos.GetStackName("i", req.InstanceName, instanceId)
	//1. 创建APP
	appId, err := aos.CreateApp(stackName, req.BlueprintId, req.Parameters, token, req.SpaceGuid)
	var res CreateInstResp
	res.Userdata = appId
	res.BaseInfo.ActualId = appId
	res.BaseInfo.InstanceType = "aos"
	res.BaseInfo.ActualName = stackName
	if err != nil {
		beego.Warn("Call AOS CreateApp fail! err:", err)
		this.Output(http.StatusInternalServerError, res)
		return
	}
	//2. 启动APP，异步的，所以直接返回。
	status, success, err := aos.StartApp(appId, token)
	if err != nil {
		beego.Warn("Call AOS StartApp fail! err:", err)
		this.Output(http.StatusInternalServerError, res)
		return
	}
	if success != true {
		beego.Warn("Call AOS StartApp fail! status:", status)
		this.Output(http.StatusInternalServerError, res)
		return
	}
	//3. 响应
	this.Output(http.StatusAccepted, res)
}

//删除 service_instances
func (this *Controller) DeleteInstance() {
	//调用AOS的API，销毁实例
	token := this.Ctx.Input.Header("X-Auth-Token")
	var req Userdatas
	//解析请求
	err := json.Unmarshal([]byte(this.Ctx.Input.RequestBody), &req)
	if err != nil {
		beego.Warn("Unmarshal delete request body fail, err:", err)
		common.OutputErrorWithCode(this.Ctx, "Unmarshal request body fail", http.StatusBadRequest)
		return
	}
	//
	appID := req.Userdata
	status, success, err := aos.DeleteApp(appID, token)
	if err != nil {
		beego.Warn("Call AOS DeleteApp fail! err:", err)
		common.OutputError(this.Ctx, err, "Call AOS DeleteApp fail! ")
		return
	}
	if success != true {
		beego.Warn("Call AOS DeleteApp fail! status:", status)
		common.OutputError(this.Ctx, err, "Call AOS DeleteApp fail! ")
		return
	}
	this.Output(http.StatusAccepted, "delete asyn")
}

//新建 service_bindings
func (this *Controller) CreateBinding() {
	var req CreateBindReq
	err := json.Unmarshal([]byte(this.Ctx.Input.RequestBody), &req)
	if err != nil {
		beego.Warn("Unmarshal CreateBinding request body fail, err:", err)
		common.OutputErrorWithCode(this.Ctx, "Unmarshal CreateBinding request body fail", http.StatusBadRequest)
		return
	}
	//这里要返回使用服务实例的账号。example直接给个fake的
	var res CreateBindResp
	credential := make(map[string]interface{})
	credential["username"] = "testUser"
	credential["paasword"] = "testPassword"
	res.Credentials = credential
	res.Userdata = req.Userdata
	this.Output(http.StatusOK, res)
}

//删除 service_bindings
func (this *Controller) DeleteBinding() {
	// |200 OK     |Binding was deleted
	this.Output(http.StatusOK, "Binding was deleted")
}

//更新 service_instance
func (this *Controller) UpdateInstance() {
	var req UpdateInstReq
	err := json.Unmarshal([]byte(this.Ctx.Input.RequestBody), &req)
	if err != nil {
		beego.Warn("Unmarshal UpdateInstance request body fail, err:", err)
		common.OutputErrorWithCode(this.Ctx, "Unmarshal UpdateInstance request body fail", http.StatusBadRequest)
		return
	}
	beego.Info("UpdateInstance request: ", req)
	//1. 构造参数
	token := this.Ctx.Input.Header("X-Auth-Token")
	appId := req.Userdata
	beego.Info(fmt.Printf("UpdateInstance request token:%d, appid:%s", len(token), appId))
	pMap := req.Parameters
	/* 目前只做了实例扩容
	if pMap != nil && len(pMap) > 0 {
		instanceNum, ok := pMap["instanceNum"].(string)
		//2. 调用AOS实例扩容接口
		if ok {
			beego.Info("get instanceNum: ", instanceNum)
			instances, err := strconv.Atoi(instanceNum)
			if err != nil {
				beego.Error("convert instanceNum error, error is: ", err)
			}
			success, err := aos.ScaleAppInstances(appId, token, instances)
			if err != nil {
				beego.Error("call Broker ScaleAppInstances error, error is: ", err)
			}
			beego.Info("Broker ScaleAppInstances success: ", success)
		}
	}*/
	// 支持所有参数的更新 by wxy
	if pMap != nil && len(pMap) > 0 {
		//2. 调用AOS实例扩容接口
		success, err := aos.UpdateInstancesInputs(appId, token, pMap)
		if err != nil {
			beego.Error("call Broker UpdateInstancesInputs error, error is: ", err)
		}
		beego.Info("Broker UpdateInstances success: ", success)
	}
	//3. 响应
	var res CreateInstResp
	res.Userdata = appId
	res.BaseInfo.ActualId = appId
	res.BaseInfo.InstanceType = "aos"
	beego.Info("UpdateInstance resp:", res)
	this.Output(http.StatusAccepted, res)
}
func getDashboard(appId, token string) string {
	// 获取服务的URI后缀
	uri := beego.AppConfig.String("service_uri")
	//访问路径
	dashboardUrl, err := aos.GetDashboardUrl(appId, token)
	if err != nil {
		beego.Warn("app getDashboard failed, error is: ", err)
	} else {
		//这里是根据Blueprint中的output章节的内容获取的:针对Container的应用这里怎么取到nodePort待确认
		//ips, _ := outputs["hostip"].([]interface{})
		//ip, _ := ips[0].(string)
		//res.Dashboard_url = "http://" + ip + ":31108/v1/vmall"
		dashboardUrl = "http://" + dashboardUrl + uri
	}
	return dashboardUrl
}

//异步查询服务实例 last_operation
func (this *Controller) LastOpertaion() {
	// 查询AOS接口，判断实例是否启动OK
	token := this.Ctx.Input.Header("X-Auth-Token")
	appId := this.Ctx.Input.Query("userdata")
	operate := this.Ctx.Input.Query("operation")
	//创建or删除
	var res LastOperationRsp
	res.Userdata = appId
	beego.Info("res.Userdata:", res.Userdata)
	if operate == "create" {
		appStatus, err := aos.QueryAppStatus(appId, token)
		if err != nil {
			beego.Warn("Query app status failed, error is: ", err)
			res.State = aos.INSTANCE_IN_PROGRESS
		} else if appStatus == aos.RUNNING {
			res.State = aos.INSTANCE_SUCCEEDED
			res.Dashboard_url = getDashboard(appId, token)
		} else if appStatus == aos.ABNORMAL {
			res.State = aos.INSTANCE_FAILED
			beego.Error(appStatus)
			this.Output(http.StatusInternalServerError, res)
			return
		} else {
			res.State = aos.INSTANCE_IN_PROGRESS
			beego.Debug(appStatus)
		}
	} else if operate == "delete" {
		success, err := aos.CheckAppDeleteSuccess(appId, token)
		if err != nil {
			beego.Warn("Check app delete status failed, error is: ", err)
			res.State = aos.INSTANCE_IN_PROGRESS
		} else if success == true {
			res.State = aos.INSTANCE_SUCCEEDED
		} else {
			res.State = aos.INSTANCE_IN_PROGRESS
		}
	} else if operate == "update" {
		appStatus, err := aos.QueryAppStatus(appId, token)
		if err != nil {
			beego.Warn("Query app status failed, error is: ", err)
			res.State = aos.INSTANCE_IN_PROGRESS
		} else if appStatus == aos.RUNNING {
			res.Dashboard_url = getDashboard(appId, token)
			res.State = aos.INSTANCE_SUCCEEDED
		} else if appStatus == aos.ABNORMAL {
			res.State = aos.INSTANCE_FAILED
			beego.Error(appStatus)
			this.Output(http.StatusInternalServerError, res)
			return
		} else {
			res.State = aos.INSTANCE_IN_PROGRESS
			beego.Debug(appStatus)
		}
	}
	beego.Info("resp:", res)
	this.Output(http.StatusOK, res)
}

//自定义订购页面
func (this *Controller) ProvisionWeb() {
	//
	backUrl := this.Ctx.Input.Query("backUrl")
	beego.Info("backUrl is:", backUrl)
	this.Ctx.Output.SetStatus(http.StatusFound)
	redirUrl := backUrl
	this.Ctx.Output.Header("Location", redirUrl)
}

//自定义更新页面
func (this *Controller) UpdateWeb() {
	backUrl := this.Ctx.Input.Query("backUrl")
	para := this.Ctx.Input.Query("preInfo")
	beego.Info("backUrl is:", backUrl)
	beego.Info("preInfo is:", para)
	this.Ctx.Output.SetStatus(http.StatusFound)
	redirUrl := backUrl
	if para != "" {
		redirUrl += "?parameters=" + para
	}
	this.Ctx.Output.Header("Location", redirUrl)
}

//-------------------------
func (this *Controller) Output(statusCode int, data interface{}) {
	this.Ctx.Output.SetStatus(statusCode)
	var result []byte
	if d, ok := data.([]byte); !ok {
		result, _ = json.Marshal(data)
	} else {
		result = d
	}
	this.Ctx.Output.Body(result)
}

// 查询实例状态
func (this *Controller) GetInstanceStatus() {
	token := this.Ctx.Input.Header("X-Auth-Token")
	//解析请求body 体
	bodyBuffer, err := ioutil.ReadAll(this.Ctx.Request.Body)
	if err != nil {
		beego.Error("GetInstanceStatus read request body error: ", err)
		common.OutputErrorWithCode(this.Ctx, "request body invalid", http.StatusBadRequest)
		return
	}
	appId := string(bodyBuffer)
	beego.Info("userdata(appId) is: ", appId)
	if appId == "" {
		beego.Error("GetInstanceStatus app id in request body empty")
		common.OutputErrorWithCode(this.Ctx, "request body invalid", http.StatusBadRequest)
		return
	}
	status, err := aos.QueryAppStatus(appId, token)
	if err != nil {
		beego.Error("query app status from aos error: ", err)
		this.Output(http.StatusInternalServerError, `{"status":"unavailable"}`)
		return
	}
	beego.Info("app status from aos is: ", status)
	if status == aos.RUNNING {
		this.Output(http.StatusOK, `{"status":"available"}`)
	} else if status == aos.APP_NOT_EXIST {
		this.Output(http.StatusGone, `{"status":"unavailable","message":"app not exist"}`)
	} else {
		this.Output(http.StatusOK, `{"status":"unavailable","message":"app status not ok"}`)
	}
}
