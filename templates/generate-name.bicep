@description('Resource Group ID for uniqueString generation')
param resourceGroupId string

var storageAccountName = 'azst${uniqueString(resourceGroupId)}'

output storageAccountName string = storageAccountName