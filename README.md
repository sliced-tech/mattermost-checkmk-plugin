# mattermost-checkmk-plugin
Mattermost Chat Plugin that integrates with Check_MK

Thanks to https://github.com/rmblake/check_mk-slack for the base of the check_mk notification script

## HOW TO USE:
### Check_MK Integration
1) Create an incoming webhook integration in your mattermost instance and note down the URL.

2) Put into /usr/(local/)share/check_mk/notifications (or ~/share/check_mk/notifications on OMD/newer check_mk installs) directory and 
edit configuration variables (slack_domain and slack_path) in the 'slack' script, and make sure that the script is executable (chmod +x slack)

3) Restart OMD/Check MK with 'omd restart' or 'cmk -R'

4) Create a user for slack in WATO, use flexible custom notifications and select 'CMK-Slack Websocket integration' as the notifier.

Select option "Call with the following parameters" and set your channel without "#". If you leave the parameter box in blank the channel takes "#monitoring" value.

5) Wait for something to send an alert or generate a test alert.

### Mattermost Integration
1) Build this project and place the resulting plugin and plugin.json in the plugins directory within mattermost under au.com.slicedtech.chat.checkmk

2) Login to mattermost, enabling the plugin, then configure the plugin under Check_MK plugin options

3) Be sure to set the URL, Username, and Secret

4) Wait for something to send an alert and then select the Acknowledge button

## Future
Hopefully adding more responses from Mattermost back to Check_MK, e.g downtime service/hosts, etc.
