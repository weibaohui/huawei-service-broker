package aos

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/astaxie/beego"
	http_client "service-broker/rest"
)

// AOS的地址
var endpoint string

func init() {
	endpoint = beego.AppConfig.String("aos_endpoint")
	beego.Info("endpoint: ", endpoint)
}

const (
	APP_ROUTER_PREFIX       = "/v2/stacks"
	APP_NAME_MAX_LENGTH     = 20
	BROKER_CREATE_OPERATION = "create"
	BROKER_UPDATE_OPERATION = "update"
	BROKER_DELETE_OPERATION = "delete"
	BROKER_TEMPLATE         = "paasbroker"
	NAME_SPACE              = "servicebroker"
	BROKER_NODE_NAME        = "paas-broker-app" // 写死在 blueprint 中
	// AOS(2017.3.28):Stack的状态目前有：Pending,Processing,Running,Stopped,PartialStopped,Abnormal,Unknow;
	// 原先 Failed状态没有了,增加PartialStopped,Abnormal,Unknow三个状态
	RUNNING = "Running"
	// FAILED        = "Failed"
	ABNORMAL                = "Abnormal" // 与aos同步，增加abnormal态
	APP_NOT_EXIST           = "app_not_exist"
	INSTANCE_IN_PROGRESS    = "in progress"
	INSTANCE_SUCCEEDED      = "succeeded"
	INSTANCE_FAILED         = "failed"
	APP_SCALE_INSTANCES_KEY = "instances"
)

type CreateAppReq struct {
	Name       string     `json:"name"`
	TemplateId string     `json:"template_id"`
	InputsJson InputsJson `json:"inputs_json"`
	ProjectId  string     `json:"project_id"`
}

// 实例参数扩容
type InputsInstanceReq struct {
	Lifecycle string                 `json:"lifecycle"`
	Inputs    map[string]interface{} `json:"inputs,omitempty" description:"Action lifecycle parameters"`
}

// 实例个数扩容
type ScaleAppInstanceReq struct {
	Lifecycle string      `json:"lifecycle"`
	Nodes     []ScaleNode `json:"nodes"`
}
type ScaleNode struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"` // {"name": "nodeid", parameters: {"instances": x}}
}

// 返回内容null,http状态码:200
type ScaleAppInstanceResp interface{}

// 输入参数格式
type InputsJson interface{}
type CreateAppResp struct {
	Guid string `json:"guid"`
}
type StartAppReq struct {
	Op        string `json:"op"`
	Path      string `json:"path"`
	Lifecycle string `json:"lifecycle"`
}
type QueryAppResp struct {
	Status string `json:"status"`
}
type AppNodeResp struct {
	RuntimeProperties map[string]interface{} `json:"runtime_properties"`
	Instances         struct {
		Items []struct {
			Status struct {
				HostIp string `json:"hostIP"`
			} `json:"status"`
		} `json:"items"`
	} `json:"instances"`
}
type AppNodeInfo struct {
	NodeId  string `json:"id"`
	InstNum int    `json:"number_of_instances,omitempty"`
	Type    string `json:"type,omitempty"`
}
type SetEnvbody struct {
	BindEnv BindEnvInfo `json:"env"`
}
type BindEnvInfo struct {
	BindServices map[string][]EnvSetEntity `json:"BIND_SERVICES"`
}
type EnvSetEntity struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Tags        []string `json:"tags"`
	Plan        string   `json:"plan"`
	Credentials string   `json:"credentials"`
}
type Outputs struct {
	Outputs map[string]Output `json:"outputs"`
}
type Output struct {
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
}

const (
	CFE_BLUEPRINT_NODETYPE = "hwpaas.nodes.ApplicationComponent"
	AOS_BLUEPRINT_NODETYPE = "hwpaas.nodes.Application"
	MODE_ADD_OPERATE       = "ADD"
	MODE_DEL_OPERATE       = "DEL"
)

func GetStackName(prefix string, serviceName string, instanceId string) string {
	if len(serviceName) > 12 {
		serviceName = serviceName[0:12]
	}
	var stackName string
	if len(instanceId) <= 5 {
		stackName = prefix + "-" + serviceName + "-" + instanceId
	} else {
		stackName = prefix + "-" + serviceName + "-" + string(instanceId[0:5])
	}
	return strings.TrimRight(stackName, "-")
}

// 注意: 服务名称较短，这里截断后要防止出现名称重复, 前端要注意限制长度, 尾部字符不能是 -
func GetAppFinalName(appName string) string {
	if len(appName) > APP_NAME_MAX_LENGTH {
		rs := []rune(appName)
		appName = string(rs[0:APP_NAME_MAX_LENGTH])
	}
	return strings.TrimRight(appName, "-")
}
func CreateApp(appName, templateId string, inputsJson InputsJson, token, projectId string) (appId string, err error) {
	path := APP_ROUTER_PREFIX
	var appReq CreateAppReq
	appReq.Name = appName
	appReq.TemplateId = templateId
	appReq.InputsJson = inputsJson
	appReq.ProjectId = projectId
	body, err := json.Marshal(appReq)
	if err != nil {
		beego.Error("Create application marshal request body error, error is: ", err)
		return
	}
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	params := make(map[string]string)
	resp, err := http_client.DoHTTPrequest("POST", endpoint, path, headers, params, body)
	if err != nil {
		beego.Error("Create application marshal request body error, error is: ", err)
		return
	}
	appRespBody, err := http_client.CopyResponseBody(resp)
	if err != nil {
		beego.Error("Create application copy response body error, error is: ", err)
		return
	}
	if !http_client.IsResponseStatusOk(resp) {
		err = errors.New("Create app from cfe error: " + string(appRespBody))
		return
	}
	var appResp CreateAppResp
	err = json.Unmarshal(appRespBody, &appResp)
	if err != nil {
		beego.Error("Create application unmarshal response body error, error is: ", err)
		return
	}
	return appResp.Guid, nil
}

// 服务实例参数更新
func UpdateInstancesInputs(appId, token string, inputs map[string]interface{}) (success bool, err error) {
	beego.Info(fmt.Printf("UpdateInstancesInputs appid:%s, inputs:%d", appId, inputs))
	path := APP_ROUTER_PREFIX + "/" + appId + "/actions"
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	var inputsReq InputsInstanceReq
	inputsReq.Lifecycle = "upgrade"
	inputsReq.Inputs = inputs
	reqBody, err := json.Marshal(inputsReq)
	params := make(map[string]string)
	if err != nil {
		beego.Error("UpdateInstancesInputs marshal request body error, error is: ", err)
		return
	}
	response, err := http_client.DoHTTPrequest("PUT", endpoint, path, headers, params, reqBody)
	if err != nil {
		beego.Error("UpdateInstancesInputs error, error is: ", err)
		return
	}
	respBody, err := http_client.CopyResponseBody(response)
	if err != nil {
		beego.Error("UpdateInstancesInputs copy response body error, error is: ", err)
		return
	}
	beego.Info("UpdateInstancesInputs response body: ", string(respBody))
	if !http_client.IsResponseStatusOk(response) {
		err = errors.New("UpdateInstancesInputs from AOS error: " + string(respBody))
		return
	}
	return true, nil
}
func SetAppEnv(appId, nodeId, parameters, token string) (success bool, err error) {
	path := APP_ROUTER_PREFIX + "/" + appId + "/nodes/" + nodeId + "/properties"
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	resp, err := http_client.DoHTTPrequest("PUT", endpoint, path, headers, nil, []byte(parameters))
	if err != nil {
		beego.Error("Set application env app id: "+appId+" node id: "+nodeId+" do request error, error is: ", err)
		return
	}
	if http_client.IsResponseStatusOk(resp) {
		success = true
		http_client.CloseResponseBody(resp)
		return
	} else {
		var appRespBody []byte
		appRespBody, err = http_client.CopyResponseBody(resp)
		if err != nil {
			beego.Error("Set application env app id: "+appId+" node id: "+nodeId+" copy response body error, error is: ", err)
			return
		} else {
			err = errors.New("Set app env, app id: " + appId + ", node id: " + nodeId + " ,error: " + string(appRespBody))
			return
		}
	}
}
func StartApp(appId, token string) (status int, success bool, err error) {
	startAppReq := StartAppReq{
		Op:        "replace",
		Path:      "/spec/lifecycle",
		Lifecycle: "create",
	}
	path := APP_ROUTER_PREFIX + "/" + appId + "/actions"
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	params := make(map[string]string)
	body, err := json.Marshal(startAppReq)
	if err != nil {
		beego.Error("Start application marshal request body error, error is: ", err)
		return http.StatusBadRequest, success, err
	}
	resp, err := http_client.DoHTTPrequest("PUT", endpoint, path, headers, params, body)
	if err != nil {
		beego.Error("Start application do request error, error is: ", err)
		return http.StatusInternalServerError, success, err
	}
	if http_client.IsResponseStatusOk(resp) {
		success = true
		http_client.CloseResponseBody(resp)
		return resp.StatusCode, success, nil
	} else {
		var appRespBody []byte
		appRespBody, err = http_client.CopyResponseBody(resp)
		if err != nil {
			beego.Error("Start application copy response body error, error is: ", err)
			return resp.StatusCode, success, err
		} else {
			err = errors.New("Start app error: " + string(appRespBody))
			return resp.StatusCode, success, err
		}
	}
}
func QueryAppStatus(appId, token string) (status string, err error) {
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	params := make(map[string]string)
	path := APP_ROUTER_PREFIX + "/" + appId
	var queryAppResp QueryAppResp
	// 先用最外层的 status 来判断，后续根据应用编排组的修改来改
	resp, err := http_client.DoHTTPrequest("GET", endpoint, path, headers, params, []byte(""))
	if err != nil {
		beego.Error("Check application status do request error, error is: ", err)
		return
	}
	appRespBody, err := http_client.CopyResponseBody(resp)
	if err != nil {
		beego.Error("Check application status copy response body error, error is: ", err)
		return
	}
	if !http_client.IsResponseStatusOk(resp) {
		// err = errors.New("Query app status from cfe error: " + string(appRespBody))
		beego.Error("Query app status from cfe error: " + string(appRespBody))
		if resp.StatusCode == http.StatusNotFound {
			status = APP_NOT_EXIST
		}
		return
	}
	err = json.Unmarshal(appRespBody, &queryAppResp)
	if err != nil {
		beego.Error("Check application status unmarshal response body error, error is: ", err)
		return
	}
	return queryAppResp.Status, nil
}
func DeleteApp(appId, token string) (status int, success bool, err error) {
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	params := make(map[string]string)
	path := APP_ROUTER_PREFIX + "/" + appId
	resp, err := http_client.DoHTTPrequest("DELETE", endpoint, path, headers, params, []byte(""))
	if err != nil {
		beego.Error("Delete application do request error, error is: ", err)
		return http.StatusInternalServerError, success, err
	} else if resp.StatusCode == 404 || resp.StatusCode == 410 || resp.StatusCode/100*100 == http.StatusOK {
		success = true
		http_client.CloseResponseBody(resp)
		return resp.StatusCode, success, nil
	} else {
		var body []byte
		body, err = http_client.CopyResponseBody(resp)
		if err != nil {
			return resp.StatusCode, success, err
		}
		err = errors.New("Delete app error: " + string(body))
		return resp.StatusCode, success, err
	}
}

// 只有真正返回 404 才认为不存在了
func CheckAppDeleteSuccess(appId, token string) (success bool, err error) {
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	params := make(map[string]string)
	path := APP_ROUTER_PREFIX + "/" + appId
	resp, err := http_client.DoHTTPrequest("GET", endpoint, path, headers, params, []byte(""))
	if err != nil {
		beego.Error("Check app delete status do request error, error is: ", err)
		return false, err
	}
	http_client.CloseResponseBody(resp)
	if resp.StatusCode == http.StatusNotFound {
		beego.Info("App " + appId + " delete success")
		return true, nil
	} else {
		err = errors.New("App " + appId + " still exists")
		return false, err
	}
}

// 根据编排接口获取节点信息，返回错误码和错误信息
func GetNodeId(appId, token string) (nodeId string, err error) {
	nodeSet, err := GetNodeIds(appId, token)
	if nil != err {
		beego.Error("GetNodeIds error, error is: ", err)
		return "", err
	}
	if len(nodeSet) > 0 {
		return nodeSet[0].NodeId, nil
	}
	return "", errors.New("get application nodeid")
}
func GetNodeIds(appId, token string) (nodeSet []AppNodeInfo, err error) {
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	params := map[string]string{"node_type": AOS_BLUEPRINT_NODETYPE}
	path := APP_ROUTER_PREFIX + "/" + appId + "/nodes"
	resp, err := http_client.DoHTTPrequest("GET", endpoint, path, headers, params, []byte(""))
	if nil != err {
		beego.Error("Get application nodeId error, error is: ", err)
		return nil, err
	}
	if http_client.IsResponseStatusOk(resp) {
		respBody, err := http_client.CopyResponseBody(resp)
		beego.Info("response of get nodes: ", resp.StatusCode, string(respBody))
		if err != nil {
			return nil, errors.New("fail to get node: " + err.Error())
		}
		err = json.Unmarshal([]byte(respBody), &nodeSet)
		if err != nil {
			return nil, errors.New("fail to unmarshal node response body: " + err.Error())
		}
		return nodeSet, nil
	}
	return nil, errors.New("invalid response(do request to get application nodeids): " + strconv.Itoa(resp.StatusCode))
}
func GetEnv(appId, nodeId, token string) (envBody SetEnvbody, err error) {
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	var path string
	if nodeId == "" {
		return envBody, errors.New("temporarily not support for nodeGuid is empty")
	}
	if nodeId != "" {
		path = APP_ROUTER_PREFIX + "/" + appId + "/nodes/" + nodeId + "/properties"
	}
	return queryBindEnv(appId, nodeId, path, token)
}

// 调用编排接口获取现有的环境变量: 内部使用
func queryBindEnv(appId string, nodeId string, path string, token string) (SetEnvbody, error) {
	var envBody SetEnvbody
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	resp, err := http_client.DoHTTPrequest("GET", "", "", nil, nil, nil)
	if err != nil {
		return envBody, errors.New("do request to get env error: " + err.Error())
	}
	if http_client.IsResponseStatusOk(resp) {
		respBody, err := http_client.CopyResponseBody(resp)
		beego.Info("response of get env: ", resp.StatusCode, string(respBody), len(respBody))
		if err != nil {
			return envBody, errors.New("fail to get env response body: " + err.Error())
		}
		if len(respBody) > 0 {
			err = json.Unmarshal(respBody, &envBody)
			if err != nil {
				return envBody, errors.New("fail to unmarshal env response body: " + err.Error())
			}
		}
	} else {
		statusCode := strconv.Itoa(resp.StatusCode)
		respBody, err := http_client.CopyResponseBody(resp)
		if err != nil {
			beego.Error("invalid response(do request to get env), status code: ", statusCode, " copy respnse body error:", err)
		}
		return envBody, errors.New("invalid response(do request to get env), status code: " + statusCode + " response body :" + string(respBody))
	}
	return envBody, nil
}
func searchInstName(instancesEnv []EnvSetEntity, instName string) int {
	if len(instancesEnv) > 0 {
		for index, item := range instancesEnv {
			if item.Name == instName {
				return index
			}
		}
	}
	return -1
}

// 修改环境变量: 内部使用
func modifyBindEnv(serviceName string, envBody SetEnvbody, envItem EnvSetEntity, mode string) SetEnvbody {
	instName := envItem.Name
	if mode == "ADD" {
		if envBody.BindEnv.BindServices == nil {
			envBody.BindEnv.BindServices = make(map[string][]EnvSetEntity)
		}
		instancesEnv := envBody.BindEnv.BindServices[serviceName]
		if instancesEnv == nil {
			envBody.BindEnv.BindServices[serviceName] = []EnvSetEntity{} // 初始化切片
			instancesEnv = envBody.BindEnv.BindServices[serviceName]
		}
		// 搜索一下是否存在, 如果不存在则添加; 如果已存在给出warning信息后直接返回
		idx := searchInstName(instancesEnv, instName)
		if idx >= 0 {
			beego.Warning("env for instance '" + instName + "' has been added yet!")
			return envBody
		}
		envBody.BindEnv.BindServices[serviceName] = append(instancesEnv, envItem)
	} else if mode == "DEL" {
		if envBody.BindEnv.BindServices == nil {
			beego.Warning("env is empty!")
			return envBody
		}
		instancesEnv := envBody.BindEnv.BindServices[serviceName]
		if instancesEnv == nil {
			beego.Warning("env for service '" + serviceName + "' has no env!")
			return envBody
		}
		// 搜索一下是否存在, 如果原本就不存在则无需再删除，给出提示信息后返回; 如果存在就删除
		idx := searchInstName(instancesEnv, instName)
		if idx == -1 {
			beego.Warning("env for instance '" + instName + "' has been deleted yet!")
			return envBody
		}
		envBody.BindEnv.BindServices[serviceName] = append(instancesEnv[:idx], instancesEnv[idx+1:]...)
	}
	return envBody
}
func SetCallerEnv(appId string, nodeId string, serviceName string, envItem EnvSetEntity, token string, mode string) error {
	path := APP_ROUTER_PREFIX + "/" + appId + "/properties"
	if nodeId != "" {
		path = APP_ROUTER_PREFIX + "/" + appId + "/nodes/" + nodeId + "/properties"
	}
	if nodeId == "" {
		return errors.New("temporarily not support for nodeGuid is empty")
	}
	beego.Info("endpoint:", endpoint, "path:", path, "appId", appId, "nodeId:", nodeId)
	// 查询环境变量
	envBody, err := queryBindEnv(appId, nodeId, path, token)
	if err != nil {
		return err
	}
	// 	调整环境变量并调用cfe接口
	envBody = modifyBindEnv(serviceName, envBody, envItem, mode)
	modifiedEnvBody, err := json.Marshal(envBody)
	beego.Info("envBody:", envBody, "modifiedEnvBody:", string(modifiedEnvBody))
	if err != nil {
		return errors.New("fail to marshal modified env: " + err.Error())
	}
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	resp, err := http_client.DoHTTPrequest("PUT", "", "", nil, nil, nil)
	if err != nil {
		return errors.New("do request to put env error: " + err.Error())
	}
	if http_client.IsResponseStatusOk(resp) {
		http_client.CloseResponseBody(resp)
	} else {
		statusCode := strconv.Itoa(resp.StatusCode)
		respBody, err := http_client.CopyResponseBody(resp)
		if err != nil {
			beego.Error("fail to put env response body, status code: ", statusCode, " copy respnse body error:", err)
		}
		return errors.New("fail to put env response body, status code:" + statusCode + "response body :" + string(respBody))
	}
	return nil
}
func GetDashboardUrl(appId string, token string) (url string, err error) {
	nodeId, err := GetNodeId(appId, token)
	if err != nil {
		beego.Error("Do request GetNodeId error: ", err)
		return "", err
	}
	beego.Info("nodeId:", nodeId)
	_, hostIp, err := QueryAppIp(appId, nodeId, token)
	if err != nil {
		beego.Error("Do QueryAppIp error: ", err)
		return "", err
	}
	// 通过获取node得到的port是创建时的port，更新实例后的port会改变因此通过output获取port，临时规避--by wxy
	outputs, err := GetBlueprintOutput(appId, token)
	if port, ok := outputs["address_port"].(string); ok {
		url = hostIp + ":" + port
		beego.Info("port:", port)
	} else {
		url = hostIp + ":" + strconv.Itoa(outputs["address_port"].(int))
	}
	// url = hostIp + ":" + strconv.Itoa(port)
	beego.Info("url:", url)
	return url, nil
}
func GetBlueprintOutput(appId string, token string) (map[string]interface{}, error) {
	path := APP_ROUTER_PREFIX + "/" + appId + "/outputs"
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	resp, err := http_client.DoHTTPrequest("GET", endpoint, path, headers, nil, nil)
	if err != nil {
		beego.Error("Do request (get blueprint output) error: ", err)
		return nil, err
	}
	respBody, err := http_client.CopyResponseBody(resp)
	if err != nil {
		beego.Error("Read response body (get blueprint output) error: ", err)
		return nil, err
	}
	// 检查返回结果是否正常
	if !http_client.IsResponseStatusOk(resp) {
		err = errors.New("Response status code (get blueprint output) invalid: " + string(respBody))
		return nil, err
	}
	var outputs Outputs
	err = json.Unmarshal(respBody, &outputs)
	beego.Info("outputs:", outputs)
	if err != nil {
		beego.Error("Unmarshall blueprint's output error: ", err)
		return nil, err
	}
	// 数据转换	map[string]Output to map[string][string], 其中 Output 的 description 信息会丢弃掉.
	dest := make(map[string]interface{})
	for k, v := range outputs.Outputs {
		dest[k] = v.Value
	}
	return dest, nil
}
func QueryAppIp(appId, nodeId, token string) (port int, hostIp string, err error) {
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	params := make(map[string]string)
	path := APP_ROUTER_PREFIX + "/" + appId + "/nodes/" + nodeId
	var nodeResp AppNodeResp
	resp, err := http_client.DoHTTPrequest("GET", endpoint, path, headers, params, []byte(""))
	if err != nil {
		beego.Error("Query App Host IP do request error, error is: ", err)
		return
	}
	nodeRespBody, err := http_client.CopyResponseBody(resp)
	if err != nil {
		beego.Error("Query App Host IP copy response body error, error is: ", err)
		return
	}
	if !http_client.IsResponseStatusOk(resp) {
		err = errors.New("Create app host ip from cfe error: " + string(nodeRespBody))
		return
	}
	err = json.Unmarshal(nodeRespBody, &nodeResp)
	if err != nil {
		beego.Error("Query App Host IP unmarshal response body error, error is: ", err)
		return
	}
	beego.Debug("the ans is : ", nodeResp)
	var service map[string]interface{}
	if nil != nodeResp.RuntimeProperties["Service"] {
		service = nodeResp.RuntimeProperties["Service"].(map[string]interface{})
	} else {
		beego.Error("The service info is:", nodeResp.RuntimeProperties["Service"], "  error:", err)
		err = errors.New("The service info is nil")
		return
	}
	// 获取app的port
	beego.Debug("The servicePort is:", service["ports"])
	servicePort := service["ports"].([]interface{})
	var nodePort map[string]interface{}
	if len(servicePort) > 0 {
		nodePort = servicePort[0].(map[string]interface{})
	} else {
		err = errors.New("servicePort Ports is null")
		return
	}
	beego.Debug("The nodePort is:", nodePort)
	port = int(nodePort["nodePort"].(float64))
	if len(nodeResp.Instances.Items) > 0 {
		hostIp = nodeResp.Instances.Items[0].Status.HostIp
		return
	} else {
		err = errors.New("App node format is illegal")
		return
	}
}
func Reconfigure(appId string, token string) error {
	headers := make(map[string]string)
	headers["X-Auth-Token"] = token
	path := APP_ROUTER_PREFIX + "/" + appId + "/actions"
	bodyMap := make(map[string]interface{})
	bodyMap["lifecycle"] = "reconfigure"
	data, _ := json.Marshal(bodyMap)
	fmt.Sprintln(path, data)

	resp, err := http_client.DoHTTPrequest("PUT", "", "", nil, nil, nil)
	if err != nil {
		return errors.New("do request (put reconfigure) error: " + err.Error())
	}
	respBody, err := http_client.CopyResponseBody(resp)
	if err != nil {
		beego.Error("Read response body (put reconfigure) error: ", err)
		return err
	}
	if !http_client.IsResponseStatusOk(resp) {
		return errors.New("Response status code (put reconfigure) is invalid: " + string(respBody))
	}
	return nil
}
