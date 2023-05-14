export type Client = {
    address: string,
    lastHeartbeatTime: string,
    lastHeartbeat:{
        "StrainId": string,
        "InitTime": string,
        "Modules": string[],
        "HostOS": string,
        "HostArch": string,
        "Hostname": string,
        "HostUser": string,
        "HostUserId": string,
        "Errors": number
    }
}