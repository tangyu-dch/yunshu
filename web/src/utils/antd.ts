import { App, message as staticMessage, notification as staticNotification, Modal as staticModal } from 'antd'
import type { MessageInstance } from 'antd/es/message/interface'
import type { NotificationInstance } from 'antd/es/notification/interface'

let message: MessageInstance = staticMessage
let notification: NotificationInstance = staticNotification
let modal: any = staticModal

export function AntdStaticHelper() {
  const app = App.useApp()
  message = app.message
  notification = app.notification
  modal = app.modal
  return null
}

export { message, notification, modal }
