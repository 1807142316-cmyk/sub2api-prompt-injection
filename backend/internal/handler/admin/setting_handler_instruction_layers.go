package admin

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetInstructionLayers is mounted only under the authenticated administrator
// settings router. The service resolves request identities independently.
func (h *SettingHandler) GetInstructionLayers(c *gin.Context) {
	config, err := h.settingService.GetInstructionLayers(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *SettingHandler) UpdateInstructionLayers(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxInstructionLayersConfigBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var request *service.InstructionLayersConfig
	if err := decoder.Decode(&request); err != nil || request == nil {
		response.BadRequest(c, "三层指令请求必须是有效的配置对象，且不超过 2 MiB")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.BadRequest(c, "三层指令请求包含多余的 JSON 内容")
		return
	}
	config, err := h.settingService.UpdateInstructionLayers(c.Request.Context(), *request)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}
