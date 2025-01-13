// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package smtp

import (
	"crypto/tls"
	"os"
	"path/filepath"
	_ "path/filepath"
	"testing"

	"github.com/gopkg.in/gomail.v2"
)

func Test1(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	to := "m@medcl.net"
	cc := []string{}
	//cc := []string{"liaosy@infinilabs.com"}

	//licenseStr:=""
	subject := "请查收您的免费授权信息! [INFINI Labs]"
	//htmlContent := "" +
	//	"<html xmlns=\"http://www.w3.org/1999/xhtml\" xmlns:o=\"urn:schemas-microsoft-com:office:office\" xmlns:v=\"urn:schemas-microsoft-com:vml\" lang=\"en\"><head>\n    <title></title>\n    <meta property=\"og:title\" content=\"\">\n    <meta name=\"twitter:title\" content=\"\">\n    \n    \n    \n<meta name=\"x-apple-disable-message-reformatting\">\n<meta http-equiv=\"Content-Type\" content=\"text/html; charset=UTF-8\">\n\n<meta http-equiv=\"X-UA-Compatible\" content=\"IE=edge\">\n\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n\n\n    <!--[if gte mso 9]>\n  <xml>\n      <o:OfficeDocumentSettings>\n      <o:AllowPNG/>\n      <o:PixelsPerInch>96</o:PixelsPerInch>\n      </o:OfficeDocumentSettings>\n  </xml>\n  \n  <style>\n    ul > li {\n      text-indent: -1em;\n    }\n  </style>\n<![endif]-->\n<!--[if mso]>\n<style type=\"text/css\">\n body, td {font-family: Arial, Helvetica, sans-serif;} \n .hse-body-wrapper-table {background-color: #EAF0F6;padding: 20px 0 !important}\n</style>\n<![endif]-->\n\n\n    \n  <meta name=\"generator\" content=\"HubSpot\">\n  <style type=\"text/css\">.moz-text-html .hse-column-container{max-width:600px !important;width:600px !important}\n.moz-text-html .hse-column{display:table-cell;vertical-align:top}.moz-text-html .hse-section .hse-size-12{max-width:600px !important;width:600px !important}\n[owa] .hse-column-container{max-width:600px !important;width:600px !important}[owa] .hse-column{display:table-cell;vertical-align:top}\n[owa] .hse-section .hse-size-12{max-width:600px !important;width:600px !important}\n@media only screen and (min-width:640px){.hse-column-container{max-width:600px !important;width:600px !important}\n.hse-column{display:table-cell;vertical-align:top}.hse-section .hse-size-12{max-width:600px !important;width:600px !important}\n}@media only screen and (max-width:639px){img.stretch-on-mobile,.hs_rss_email_entries_table img,.hs-stretch-cta .hs-cta-img{height:auto !important;width:100% !important}\n.display_block_on_small_screens{display:block}.hs_padded{padding-left:20px !important;padding-right:20px !important}\n}</style><!--<![endif]--><style type=\"text/css\">body[data-outlook-cycle] img.stretch-on-mobile,body[data-outlook-cycle] .hs_rss_email_entries_table img{height:auto !important;width:100% !important}\nbody[data-outlook-cycle] .hs_padded{padding-left:20px !important;padding-right:20px !important}\na[x-apple-data-detectors]{color:inherit !important;text-decoration:none !important;font-size:inherit !important;font-family:inherit !important;font-weight:inherit !important;line-height:inherit !important}\n#outlook a{padding:0}.yshortcuts a{border-bottom:none !important}a{text-decoration:underline}\n.ExternalClass{width:100%}.ExternalClass,.ExternalClass p,.ExternalClass td,.ExternalClass div,.ExternalClass span,.ExternalClass font{line-height:100%}\np{margin:0}body{-ms-text-size-adjust:100%;-webkit-text-size-adjust:100%;-webkit-font-smoothing:antialiased;moz-osx-font-smoothing:grayscale}</style>\n</head>\n  <body bgcolor=\"#EAF0F6\" style=\"margin:0 !important; padding:0 !important; font-family:Arial, sans-serif; font-size:15px; color:#23496d; word-break:break-word\">\n    \n\n\n\n\n\n    \n<!--[if gte mso 9]>\n<v:background xmlns:v=\"urn:schemas-microsoft-com:vml\" fill=\"t\">\n  \n    <v:fill type=\"tile\" size=\"100%,100%\"  color=\"#ffffff\"/>\n  \n</v:background>\n<![endif]-->\n\n\n    <div class=\"hse-body-background\" style=\"background-color:#eaf0f6\" bgcolor=\"#eaf0f6\">\n      <table role=\"presentation\" class=\"hse-body-wrapper-table\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-spacing:0 !important; border-collapse:collapse; mso-table-lspace:0pt; mso-table-rspace:0pt; margin:0; padding:0; width:100% !important; min-width:320px !important; height:100% !important\" width=\"100%\" height=\"100%\">\n        <tbody><tr>\n          <td class=\"hse-body-wrapper-td\" valign=\"top\" style=\"border-collapse:collapse; mso-line-height-rule:exactly; font-family:Arial, sans-serif; font-size:15px; color:#23496d; word-break:break-word\">\n            <div id=\"hs_cos_wrapper_main\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_dnd_area\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"dnd_area\">  <div id=\"section-1\" class=\"hse-section hse-section-first\" style=\"padding-left:10px; padding-right:10px; padding-top:20px\">\n\n    \n    \n    <!--[if !((mso)|(IE))]><!-- -->\n      <div class=\"hse-column-container\" style=\"min-width:280px; max-width:600px; width:100%; Margin-left:auto; Margin-right:auto; border-collapse:collapse; border-spacing:0; background-color:#ffffff; padding-bottom:30px; padding-top:30px\" bgcolor=\"#ffffff\">\n    <!--<![endif]-->\n    \n    <!--[if (mso)|(IE)]>\n      <div class=\"hse-column-container\" style=\"min-width:280px;max-width:600px;width:100%;Margin-left:auto;Margin-right:auto;border-collapse:collapse;border-spacing:0;\">\n      <table align=\"center\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px;\" cellpadding=\"0\" cellspacing=\"0\" role=\"presentation\" width=\"600\" bgcolor=\"#ffffff\">\n      <tr style=\"background-color:#ffffff;\">\n    <![endif]-->\n\n    <!--[if (mso)|(IE)]>\n  <td valign=\"top\" style=\"width:600px;padding-bottom:30px; padding-top:30px;\">\n<![endif]-->\n<!--[if gte mso 9]>\n  <table role=\"presentation\" width=\"600\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px\">\n<![endif]-->\n<div id=\"column-1-0\" class=\"hse-column hse-size-12\">\n  <div id=\"hs_cos_wrapper_module_16873376536522\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_module\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"module\">\n\n\n\n\n  \n\n\n<table class=\"hse-image-wrapper\" role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-spacing:0 !important; border-collapse:collapse; mso-table-lspace:0pt; mso-table-rspace:0pt\">\n    <tbody>\n        <tr>\n            <td align=\"center\" valign=\"top\" style=\"border-collapse:collapse; mso-line-height-rule:exactly; font-family:Arial, sans-serif; color:#23496d; word-break:break-word; text-align:center;  font-size:0px\">\n                \n                <img alt=\"email-header\" src=\"https://infinilabs.com/img/email/email-header.png\" style=\"outline:none; text-decoration:none; -ms-interpolation-mode:bicubic; max-width:100%; font-size:16px\" width=\"560\" align=\"middle\">\n                \n            </td>\n        </tr>\n    </tbody>\n</table></div>\n<table role=\"presentation\" cellpadding=\"0\" cellspacing=\"0\" width=\"100%\" style=\"border-spacing:0 !important; border-collapse:collapse; mso-table-lspace:0pt; mso-table-rspace:0pt\"><tbody><tr><td class=\"hs_padded\" style=\"border-collapse:collapse; mso-line-height-rule:exactly; font-family:Arial, sans-serif; font-size:15px; color:#23496d; word-break:break-word; padding:10px 20px\"><div id=\"hs_cos_wrapper_module-1-0-0\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_module\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"module\"><div id=\"hs_cos_wrapper_module-1-0-0_\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_rich_text\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"rich_text\">\n  <p style=\"mso-line-height-rule:exactly; line-height:175%; margin-bottom:10px\">\n\n尊敬的用户，<br/>\n感谢您申请我们的免费授权！欢迎加入我们的大家庭。我代表整个团队向您表示热烈的欢迎！<br/><br/>\n\n我们非常高兴能为您提供我们产品的免费版本，让您能够体验和享受我们的服务。\n\n</p>\n\n\n<h2>\n您的 License 如下:\n</h2>\n\n<p style=\"word-break: break-all; text-wrap: wrap;background: #000000; color: #ffffff; padding: 10px;\">\n" +
	//	licenseStr+
	//	"</p>\n\n\n<p>\n<br/>\n如果您在使用过程中遇到任何问题或有任何建议，都请随时与我们联系。我们将全力以赴为您提供最好的服务。\n\n<ul>\n<li>电话：400-139-9200</li>\n<li>邮件：hello@infini.ltd</li>\n<li>网站：<a href=\"https://infinilabs.com\">infinilabs.com</a></li>\n</ul>\n\n再次感谢您选择我们的产品。我们期待着与您共同创造美好的数据分析体验！<br/>\n\n\n\n<p/>\n\n<h2>\n了解更多:\n</h2>\n\n<ol style=\"mso-line-height-rule:exactly; line-height:175%\">\n\n<li style=\"mso-line-height-rule:exactly\"><a href=\"https://infinilabs.com/en/docs/latest/gateway/tutorial/request-logging/\">Analyzing Elasticsearch slow queries becomes easier 😃</a></li>\n\n<li style=\"mso-line-height-rule:exactly\"><a href=\"https://infinilabs.com/en/docs/latest/gateway/tutorial/index_diff/\">How to differentiate Elasticsearch indices across different clusters 🍻</a></li>\n\n<li style=\"mso-line-height-rule:exactly\"><a href=\"https://infinilabs.com/en/docs/latest/console/screenshots/\">How to manage the Elasticsearch or OpenSearch clusters all together 😎</a></li>\n\n</ol>\n<p style=\"mso-line-height-rule:exactly; line-height:175%; margin-bottom:10px\">\n\n祝您使用愉快！<br/>\nINFINI Labs 团队\n\n</p></div></div></td></tr></tbody></table>\n</div>\n<!--[if gte mso 9]></table><![endif]-->\n<!--[if (mso)|(IE)]></td><![endif]-->\n\n\n    <!--[if (mso)|(IE)]></tr></table><![endif]-->\n\n    </div>\n   \n  </div>\n\n  <div id=\"section-2\" class=\"hse-section\" style=\"padding-left:10px; padding-right:10px\">\n\n    \n    \n    <!--[if !((mso)|(IE))]><!-- -->\n      <div class=\"hse-column-container\" style=\"min-width:280px; max-width:600px; width:100%; Margin-left:auto; Margin-right:auto; border-collapse:collapse; border-spacing:0; padding-bottom:20px; padding-top:20px\">\n    <!--<![endif]-->\n    \n    <!--[if (mso)|(IE)]>\n      <div class=\"hse-column-container\" style=\"min-width:280px;max-width:600px;width:100%;Margin-left:auto;Margin-right:auto;border-collapse:collapse;border-spacing:0;\">\n      <table align=\"center\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px;\" cellpadding=\"0\" cellspacing=\"0\" role=\"presentation\">\n      <tr>\n    <![endif]-->\n\n    <!--[if (mso)|(IE)]>\n  <td valign=\"top\" style=\"width:600px;padding-bottom:20px; padding-top:20px;\">\n<![endif]-->\n<!--[if gte mso 9]>\n  <table role=\"presentation\" width=\"600\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px\">\n<![endif]-->\n<div id=\"column-2-0\" class=\"hse-column hse-size-12\">\n  <div id=\"hs_cos_wrapper_module-2-0-0\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_module\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"module\">\n\n\n\n</div>\n</div>\n<!--[if gte mso 9]></table><![endif]-->\n<!--[if (mso)|(IE)]></td><![endif]-->\n\n\n    <!--[if (mso)|(IE)]></tr></table><![endif]-->\n\n    </div>\n   \n  </div>\n\n\n\n<!--[if (mso)|(IE)]></td></tr></table><![endif]-->\n\n</div>\n</div>\n</div>\n          </td>\n        </tr>\n      </tbody></table>\n    </div>\n  \n</body></html>"+
	//	""

	htmlText := "请查收您的免费授权信息! [INFINI Labs]"

	processor := SMTPProcessor{
		config: &Config{
			Templates: map[string]*Template{},
			Servers: map[string]*ServerConfig{
				"notify-test": {
					SendFrom: "notify-test@infini.ltd",
					Auth: struct {
						Username string `config:"username"`
						Password string `config:"password"`
					}(struct {
						Username string
						Password string
					}{Username: "notify-test@infini.ltd", Password: "XXXX"}),
					Server: struct {
						Host string `config:"host"`
						Port int    `config:"port"`
						TLS  bool   `config:"tls"`
					}(struct {
						Host string
						Port int
						TLS  bool
					}{Host: "smtp.ym.163.com", Port: 994, TLS: true})},
			},
		},
	}

	processor.send(processor.config.Servers["notify-test"], []string{to}, cc, subject, "text/plain", htmlText, nil)
	//processor.send(to,cc,subject,"text/html",htmlContent)

}
func Test2(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	from := "notify-test@infini.ltd"
	to := "medcl@infinilabs.com"
	cc := []string{"liaosy@infinilabs.com", "Liaosy"}

	//license :="curl -XPOST http://localhost:2900/_license/apply -d '{\"license\":\"ewogICJsaWNlbnNlX3R5cGUiOiAiRXZhbHVhdGlvbiIsCiAgImxpY2Vuc2VfaWQiOiAiY2kwb3BncnE1MGtjMXBwdmcxNDAiLAogICJpc3N1ZV90byI6ICJsaWFvc3lAaW5maW5pbGFicy5jb20iLAogICJpc3N1ZV9hdCI6ICIyMDIzLTA2LTAxVDIwOjI5OjIxIiwKICAidmFsaWRfZnJvbSI6ICIyMDIzLTA2LTAxVDEzOjE2OjIxIiwKICAiZXhwaXJlX2F0IjogIjIwMjMtMDgtMDFUMTM6MTY6MjEiLAogICJzaWduYXR1cmUiOiAiMzljN2NiODVmZGJjZTRkMGZjMTNlYTVjMGQ2MTEyZTM4MzJjYzk0NTEwYzRjN2MwNzU3ZDcwYzMyOWQ0NzM5YjYxNDZhMTFiYTBiMDg5YjExMTExODA1ODAxNGRjNDI2ZGRhZDM2MmExNDhmYTYyYjE0MTg5MDVlNjhlYmQ1NWE4YjljM2M5MjhhNzY5NGJjNzBjODQ1ZjMxYzFiYWMxZDFjM2YwZDBiY2I3Y2VjMTc0NjA2MjIzZTdhMzMzZmEyMmMwMDFmYWJiMjc0NWEzZmM3ZTBjZmQyMWVlYmQ1N2ZjYWMyMzJiNjliNjYzMzUzMzc3Y2FmNjI3OTk5MzM1YyIsCiAgIm1heF9ub2RlcyI6IDk5OQogfQ==\"}'"

	licenseStr := ""

	// Email subject and content
	subject := "请查收您的免费授权信息! [INFINI Labs]"
	htmlContent := "" +
		"<html xmlns=\"http://www.w3.org/1999/xhtml\" xmlns:o=\"urn:schemas-microsoft-com:office:office\" xmlns:v=\"urn:schemas-microsoft-com:vml\" lang=\"en\"><head>\n    <title></title>\n    <meta property=\"og:title\" content=\"\">\n    <meta name=\"twitter:title\" content=\"\">\n    \n    \n    \n<meta name=\"x-apple-disable-message-reformatting\">\n<meta http-equiv=\"Content-Type\" content=\"text/html; charset=UTF-8\">\n\n<meta http-equiv=\"X-UA-Compatible\" content=\"IE=edge\">\n\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n\n\n    <!--[if gte mso 9]>\n  <xml>\n      <o:OfficeDocumentSettings>\n      <o:AllowPNG/>\n      <o:PixelsPerInch>96</o:PixelsPerInch>\n      </o:OfficeDocumentSettings>\n  </xml>\n  \n  <style>\n    ul > li {\n      text-indent: -1em;\n    }\n  </style>\n<![endif]-->\n<!--[if mso]>\n<style type=\"text/css\">\n body, td {font-family: Arial, Helvetica, sans-serif;} \n .hse-body-wrapper-table {background-color: #EAF0F6;padding: 20px 0 !important}\n</style>\n<![endif]-->\n\n\n    \n  <meta name=\"generator\" content=\"HubSpot\">\n  <style type=\"text/css\">.moz-text-html .hse-column-container{max-width:600px !important;width:600px !important}\n.moz-text-html .hse-column{display:table-cell;vertical-align:top}.moz-text-html .hse-section .hse-size-12{max-width:600px !important;width:600px !important}\n[owa] .hse-column-container{max-width:600px !important;width:600px !important}[owa] .hse-column{display:table-cell;vertical-align:top}\n[owa] .hse-section .hse-size-12{max-width:600px !important;width:600px !important}\n@media only screen and (min-width:640px){.hse-column-container{max-width:600px !important;width:600px !important}\n.hse-column{display:table-cell;vertical-align:top}.hse-section .hse-size-12{max-width:600px !important;width:600px !important}\n}@media only screen and (max-width:639px){img.stretch-on-mobile,.hs_rss_email_entries_table img,.hs-stretch-cta .hs-cta-img{height:auto !important;width:100% !important}\n.display_block_on_small_screens{display:block}.hs_padded{padding-left:20px !important;padding-right:20px !important}\n}</style><!--<![endif]--><style type=\"text/css\">body[data-outlook-cycle] img.stretch-on-mobile,body[data-outlook-cycle] .hs_rss_email_entries_table img{height:auto !important;width:100% !important}\nbody[data-outlook-cycle] .hs_padded{padding-left:20px !important;padding-right:20px !important}\na[x-apple-data-detectors]{color:inherit !important;text-decoration:none !important;font-size:inherit !important;font-family:inherit !important;font-weight:inherit !important;line-height:inherit !important}\n#outlook a{padding:0}.yshortcuts a{border-bottom:none !important}a{text-decoration:underline}\n.ExternalClass{width:100%}.ExternalClass,.ExternalClass p,.ExternalClass td,.ExternalClass div,.ExternalClass span,.ExternalClass font{line-height:100%}\np{margin:0}body{-ms-text-size-adjust:100%;-webkit-text-size-adjust:100%;-webkit-font-smoothing:antialiased;moz-osx-font-smoothing:grayscale}</style>\n</head>\n  <body bgcolor=\"#EAF0F6\" style=\"margin:0 !important; padding:0 !important; font-family:Arial, sans-serif; font-size:15px; color:#23496d; word-break:break-word\">\n    \n\n\n\n\n\n    \n<!--[if gte mso 9]>\n<v:background xmlns:v=\"urn:schemas-microsoft-com:vml\" fill=\"t\">\n  \n    <v:fill type=\"tile\" size=\"100%,100%\"  color=\"#ffffff\"/>\n  \n</v:background>\n<![endif]-->\n\n\n    <div class=\"hse-body-background\" style=\"background-color:#eaf0f6\" bgcolor=\"#eaf0f6\">\n      <table role=\"presentation\" class=\"hse-body-wrapper-table\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-spacing:0 !important; border-collapse:collapse; mso-table-lspace:0pt; mso-table-rspace:0pt; margin:0; padding:0; width:100% !important; min-width:320px !important; height:100% !important\" width=\"100%\" height=\"100%\">\n        <tbody><tr>\n          <td class=\"hse-body-wrapper-td\" valign=\"top\" style=\"border-collapse:collapse; mso-line-height-rule:exactly; font-family:Arial, sans-serif; font-size:15px; color:#23496d; word-break:break-word\">\n            <div id=\"hs_cos_wrapper_main\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_dnd_area\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"dnd_area\">  <div id=\"section-1\" class=\"hse-section hse-section-first\" style=\"padding-left:10px; padding-right:10px; padding-top:20px\">\n\n    \n    \n    <!--[if !((mso)|(IE))]><!-- -->\n      <div class=\"hse-column-container\" style=\"min-width:280px; max-width:600px; width:100%; Margin-left:auto; Margin-right:auto; border-collapse:collapse; border-spacing:0; background-color:#ffffff; padding-bottom:30px; padding-top:30px\" bgcolor=\"#ffffff\">\n    <!--<![endif]-->\n    \n    <!--[if (mso)|(IE)]>\n      <div class=\"hse-column-container\" style=\"min-width:280px;max-width:600px;width:100%;Margin-left:auto;Margin-right:auto;border-collapse:collapse;border-spacing:0;\">\n      <table align=\"center\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px;\" cellpadding=\"0\" cellspacing=\"0\" role=\"presentation\" width=\"600\" bgcolor=\"#ffffff\">\n      <tr style=\"background-color:#ffffff;\">\n    <![endif]-->\n\n    <!--[if (mso)|(IE)]>\n  <td valign=\"top\" style=\"width:600px;padding-bottom:30px; padding-top:30px;\">\n<![endif]-->\n<!--[if gte mso 9]>\n  <table role=\"presentation\" width=\"600\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px\">\n<![endif]-->\n<div id=\"column-1-0\" class=\"hse-column hse-size-12\">\n  <div id=\"hs_cos_wrapper_module_16873376536522\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_module\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"module\">\n\n\n\n\n  \n\n\n<table class=\"hse-image-wrapper\" role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-spacing:0 !important; border-collapse:collapse; mso-table-lspace:0pt; mso-table-rspace:0pt\">\n    <tbody>\n        <tr>\n            <td align=\"center\" valign=\"top\" style=\"border-collapse:collapse; mso-line-height-rule:exactly; font-family:Arial, sans-serif; color:#23496d; word-break:break-word; text-align:center;  font-size:0px\">\n                \n                <img alt=\"email-header\" src=\"https://infinilabs.com/img/email/email-header.png\" style=\"outline:none; text-decoration:none; -ms-interpolation-mode:bicubic; max-width:100%; font-size:16px\" width=\"560\" align=\"middle\">\n                \n            </td>\n        </tr>\n    </tbody>\n</table></div>\n<table role=\"presentation\" cellpadding=\"0\" cellspacing=\"0\" width=\"100%\" style=\"border-spacing:0 !important; border-collapse:collapse; mso-table-lspace:0pt; mso-table-rspace:0pt\"><tbody><tr><td class=\"hs_padded\" style=\"border-collapse:collapse; mso-line-height-rule:exactly; font-family:Arial, sans-serif; font-size:15px; color:#23496d; word-break:break-word; padding:10px 20px\"><div id=\"hs_cos_wrapper_module-1-0-0\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_module\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"module\"><div id=\"hs_cos_wrapper_module-1-0-0_\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_rich_text\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"rich_text\">\n  <p style=\"mso-line-height-rule:exactly; line-height:175%; margin-bottom:10px\">\n\n尊敬的用户，<br/>\n感谢您申请我们的免费授权！欢迎加入我们的大家庭。我代表整个团队向您表示热烈的欢迎！<br/><br/>\n\n我们非常高兴能为您提供我们产品的免费版本，让您能够体验和享受我们的服务。\n\n</p>\n\n\n<h2>\n您的 License 如下:\n</h2>\n\n<p style=\"word-break: break-all; text-wrap: wrap;background: #000000; color: #ffffff; padding: 10px;\">\n" +
		licenseStr +
		" <img width=100% height=100 id=\"1\" src=\"cid:image1.png\"></p>\n\n\n<p>\n<br/>\n如果您在使用过程中遇到任何问题或有任何建议，都请随时与我们联系。我们将全力以赴为您提供最好的服务。\n\n<ul>\n<li>电话：400-139-9200</li>\n<li>邮件：hello@infini.ltd</li>\n<li>网站：<a href=\"https://infinilabs.com\">infinilabs.com</a></li>\n</ul>\n\n再次感谢您选择我们的产品。我们期待着与您共同创造美好的数据分析体验！<br/>\n\n\n\n<p/>\n\n<h2>\n了解更多:\n</h2>\n\n<ol style=\"mso-line-height-rule:exactly; line-height:175%\">\n\n<li style=\"mso-line-height-rule:exactly\"><a href=\"https://infinilabs.com/en/docs/latest/gateway/tutorial/request-logging/\">Analyzing Elasticsearch slow queries becomes easier 😃</a></li>\n\n<li style=\"mso-line-height-rule:exactly\"><a href=\"https://infinilabs.com/en/docs/latest/gateway/tutorial/index_diff/\">How to differentiate Elasticsearch indices across different clusters 🍻</a></li>\n\n<li style=\"mso-line-height-rule:exactly\"><a href=\"https://infinilabs.com/en/docs/latest/console/screenshots/\">How to manage the Elasticsearch or OpenSearch clusters all together 😎</a></li>\n\n</ol>\n<p style=\"mso-line-height-rule:exactly; line-height:175%; margin-bottom:10px\">\n\n祝您使用愉快！<br/>\nINFINI Labs 团队\n\n</p></div></div></td></tr></tbody></table>\n</div>\n<!--[if gte mso 9]></table><![endif]-->\n<!--[if (mso)|(IE)]></td><![endif]-->\n\n\n    <!--[if (mso)|(IE)]></tr></table><![endif]-->\n\n    </div>\n   \n  </div>\n\n  <div id=\"section-2\" class=\"hse-section\" style=\"padding-left:10px; padding-right:10px\">\n\n    \n    \n    <!--[if !((mso)|(IE))]><!-- -->\n      <div class=\"hse-column-container\" style=\"min-width:280px; max-width:600px; width:100%; Margin-left:auto; Margin-right:auto; border-collapse:collapse; border-spacing:0; padding-bottom:20px; padding-top:20px\">\n    <!--<![endif]-->\n    \n    <!--[if (mso)|(IE)]>\n      <div class=\"hse-column-container\" style=\"min-width:280px;max-width:600px;width:100%;Margin-left:auto;Margin-right:auto;border-collapse:collapse;border-spacing:0;\">\n      <table align=\"center\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px;\" cellpadding=\"0\" cellspacing=\"0\" role=\"presentation\">\n      <tr>\n    <![endif]-->\n\n    <!--[if (mso)|(IE)]>\n  <td valign=\"top\" style=\"width:600px;padding-bottom:20px; padding-top:20px;\">\n<![endif]-->\n<!--[if gte mso 9]>\n  <table role=\"presentation\" width=\"600\" cellpadding=\"0\" cellspacing=\"0\" style=\"border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;width:600px\">\n<![endif]-->\n<div id=\"column-2-0\" class=\"hse-column hse-size-12\">\n  <div id=\"hs_cos_wrapper_module-2-0-0\" class=\"hs_cos_wrapper hs_cos_wrapper_widget hs_cos_wrapper_type_module\" style=\"color: inherit; font-size: inherit; line-height: inherit;\" data-hs-cos-general-type=\"widget\" data-hs-cos-type=\"module\">\n\n\n\n</div>\n</div>\n<!--[if gte mso 9]></table><![endif]-->\n<!--[if (mso)|(IE)]></td><![endif]-->\n\n\n    <!--[if (mso)|(IE)]></tr></table><![endif]-->\n\n    </div>\n   \n  </div>\n\n\n\n<!--[if (mso)|(IE)]></td></tr></table><![endif]-->\n\n</div>\n</div>\n</div>\n          </td>\n        </tr>\n      </tbody></table>\n    </div>\n  \n</body></html>" +
		""

	// Create a new message
	message := gomail.NewMessage()
	message.SetHeader("From", from)
	message.SetHeader("To", to)
	message.SetAddressHeader("Cc", cc[0], cc[1])
	message.SetHeader("Subject", subject)

	// Add HTML content to the message
	message.SetBody("text/html", htmlContent)

	//message.Attach("/Users/medcl/Desktop/WechatIMG2783.png")

	// Attach the image
	imageAttachment := "/Users/medcl/Desktop/WechatIMG2783.png"

	h := map[string][]string{
		"Content-ID":          {"image1.png"},
		"Content-Type":        {"image/png"},
		"Content-Disposition": {"attachment; filename=\"" + filepath.Base(imageAttachment) + "\""},
	}
	message.Embed(imageAttachment, gomail.SetHeader(h))

	d := gomail.NewDialer("smtp.ym.163.com", 994, from, "XXX")
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	d.SSL = true
	// Send the email to Bob, Cora and Dan.
	if err := d.DialAndSend(message); err != nil {
		panic(err)
	}
}
