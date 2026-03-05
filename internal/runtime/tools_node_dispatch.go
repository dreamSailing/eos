package runtime

import (
	"context"
	"encoding/json"
	"slices"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type DispatchToolImpl struct {
	name    string
	handler func(args map[string]any) (DispatchResult, error)
}

func (dti *DispatchToolImpl) Info(ctx context.Context) (*schema.ToolInfo, error) {
	var info *schema.ToolInfo
	for _, ti := range GetDispatchToolsInfo() {
		if ti["name"] == dti.name {
			params, _ := ti["parameters"].(map[string]any)
			props, _ := params["properties"].(map[string]any)
			req, _ := params["required"].([]string)

			pMap := make(map[string]*schema.ParameterInfo)
			for k, v := range props {
				vMap, _ := v.(map[string]any)
				pType, _ := vMap["type"].(string)
				pDesc, _ := vMap["description"].(string)

				required := slices.Contains(req, k)

				var sType schema.DataType
				switch pType {
				case "string":
					sType = schema.String
				case "integer":
					sType = schema.Integer
				case "boolean":
					sType = schema.Boolean
				case "array":
					sType = schema.Array
				case "object":
					sType = schema.Object
				}

				pMap[k] = &schema.ParameterInfo{
					Type:     sType,
					Desc:     pDesc,
					Required: required,
				}
			}

			info = &schema.ToolInfo{
				Name:        dti.name,
				Desc:        ti["description"].(string),
				ParamsOneOf: schema.NewParamsOneOfByParams(pMap),
			}
			break
		}
	}
	return info, nil
}

func (dti *DispatchToolImpl) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	args := map[string]any{}
	if err := utils.UnmarshalWithEscapeFix(argumentsInJSON, &args); err != nil {
		return "", err
	}

	result, err := dti.handler(args)
	if err != nil {
		return "", err
	}

	bs, _ := json.Marshal(result)
	return string(bs), nil
}
