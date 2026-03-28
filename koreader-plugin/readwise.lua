local InputDialog = require("ui/widget/inputdialog")
local UIManager = require("ui/uimanager")
local logger = require("logger")
local _ = require("gettext")

-- Readwise exporter with configurable server URL.
-- Fork of KOReader's built-in readwise.lua that adds a "Set server URL"
-- menu item so highlights can be sent to Marginalia or any
-- Readwise-compatible API endpoint.
--
-- upstream: https://github.com/koreader/koreader/blob/master/plugins/exporter.koplugin/target/readwise.lua
local ReadwiseExporter = require("base"):new {
    name = "readwise",
    is_remote = true,
}

local DEFAULT_SERVER = "https://readwise.io"

function ReadwiseExporter:isReadyToExport()
    if self.settings.token then return true end
    return false
end

function ReadwiseExporter:getMenuTable()
    return {
        text = _("Readwise"),
        checked_func = function() return self:isEnabled() end,
        sub_item_table = {
            {
                text = _("Set server URL"),
                keep_menu_open = true,
                callback = function()
                    local url_dialog
                    url_dialog = InputDialog:new {
                        title = _("Set Readwise-compatible server URL"),
                        input = self.settings.url or DEFAULT_SERVER,
                        hint = DEFAULT_SERVER,
                        buttons = {
                            {
                                {
                                    text = _("Cancel"),
                                    callback = function()
                                        UIManager:close(url_dialog)
                                    end
                                },
                                {
                                    text = _("Set URL"),
                                    callback = function()
                                        local input = url_dialog:getInputText()
                                        -- Strip trailing slash
                                        if input:sub(-1) == "/" then
                                            input = input:sub(1, -2)
                                        end
                                        self.settings.url = input
                                        self:saveSettings()
                                        UIManager:close(url_dialog)
                                    end
                                }
                            }
                        }
                    }
                    UIManager:show(url_dialog)
                    url_dialog:onShowKeyboard()
                end
            },
            {
                text = _("Set authorization token"),
                keep_menu_open = true,
                callback = function()
                    local auth_dialog
                    auth_dialog = InputDialog:new {
                        title = _("Set authorization token for Readwise"),
                        input = self.settings.token,
                        buttons = {
                            {
                                {
                                    text = _("Cancel"),
                                    callback = function()
                                        UIManager:close(auth_dialog)
                                    end
                                },
                                {
                                    text = _("Set token"),
                                    callback = function()
                                        self.settings.token = auth_dialog:getInputText()
                                        self:saveSettings()
                                        UIManager:close(auth_dialog)
                                    end
                                }
                            }
                        }
                    }
                    UIManager:show(auth_dialog)
                    auth_dialog:onShowKeyboard()
                end
            },
            {
                text = _("Export to Readwise"),
                checked_func = function() return self:isEnabled() end,
                callback = function() self:toggleEnabled() end,
            },

        }
    }
end

function ReadwiseExporter:createHighlights(booknotes)
    local highlights = {}

    local json_headers = {
        ["Authorization"] = "Token " .. self.settings.token,
    }

    for _, chapter in ipairs(booknotes) do
        for _, clipping in ipairs(chapter) do
            local highlight = {
                text = clipping.text,
                title = booknotes.title,
                author = booknotes.author ~= "" and booknotes.author:gsub("\n", ", ") or nil, -- optional author
                source_type = "koreader",
                category = "books",
                note = clipping.note,
                location = clipping.page,
                location_type = "order",
                highlighted_at = os.date("!%Y-%m-%dT%TZ", clipping.time),
            }
            table.insert(highlights, highlight)
        end
    end

    local base_url = self.settings.url or DEFAULT_SERVER
    local result, err = self:makeJsonRequest(base_url .. "/api/v2/highlights", "POST",
         { highlights = highlights }, json_headers)

    if not result then
        logger.warn("error creating highlights", err)
        return false
    end
    return true
end

function ReadwiseExporter:export(t)
    if not self:isReadyToExport() then return false end

    for _, booknotes in ipairs(t) do
        local ok = self:createHighlights(booknotes)
        if not ok then return false end
    end
    return true
end

return ReadwiseExporter
