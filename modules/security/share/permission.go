/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package share

type SharingPermission int

const None SharingPermission = 0    //None access
const View SharingPermission = 1    //Read-only access
const Comment SharingPermission = 2 //Can view and comment
const Edit SharingPermission = 4    //Can modify content
const Share SharingPermission = 8   //Can reshare with others
const Owner SharingPermission = 16  //Full ownership
