package http

import (
	"net/http"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/service"
)

// adminDashboardHandler 处理 GET /api/v1/admin/dashboard。
// 由 authMiddleware + roleMiddleware(super_admin, admin) 保护。
type adminDashboardHandler struct {
	svc *service.DashboardService
}

// handleDashboard 处理 Dashboard 请求。
// Query: ?range=1h|6h|24h|7d（默认 24h）。
func (h *adminDashboardHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// 校验 Method
	if r.Method != http.MethodGet {
		writeCode(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	// 解析 range 参数
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "24h"
	}

	// 校验 range 合法性
	if err := service.ValidateRange(rangeStr); err != nil {
		writeCode(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	// 校验角色（二次保险：middleware 已过滤，但 handler 再显式断言一次）
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusUnauthorized, 401, "missing claims")
		return
	}
	if claims.Role != domain.RoleSuperAdmin && claims.Role != domain.RoleAdmin {
		writeCode(w, http.StatusForbidden, 403, "admin role required")
		return
	}

	// 调用 Service 层聚合查询
	data, err := h.svc.Build(r.Context(), rangeStr)
	if err != nil {
		writeCode(w, http.StatusInternalServerError, 500, "failed to build dashboard: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
