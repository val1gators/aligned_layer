use serde::Deserialize;

#[derive(Debug, Deserialize)]
pub struct ECDSAConfig {
    pub private_key_store_path: String,
    pub private_key_store_password: String,
}

#[derive(Debug, Deserialize)]
pub struct BatcherConfigFromYaml {
    pub eth_rpc_url: String,
    pub ecdsa: ECDSAConfig,
    pub aligned_layer_deployment_config_file_path: String,
}

impl BatcherConfigFromYaml {
    pub fn new(config_file: String) -> Self {
        let config_file = std::fs::read_to_string(config_file).expect("Failed to read config file");
        serde_yaml::from_str(&config_file).expect("Failed to parse config file")
    }
}

#[derive(Debug, Deserialize)]
pub struct Addresses {
    #[serde(rename = "alignedLayerServiceManager")]
    pub aligned_layer_service_manager: String,
}

#[derive(Debug, Deserialize)]
pub struct ContractDeploymentOutput {
    pub addresses: Addresses,
}

impl ContractDeploymentOutput {
    pub fn new(deployment_output: String) -> Self {
        let deployment_output = std::fs::read_to_string(deployment_output)
            .expect("Failed to read deployment output file");
        serde_json::from_str(&deployment_output).expect("Failed to parse deployment output file")
    }
}
