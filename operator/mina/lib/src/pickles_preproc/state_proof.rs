use kimchi::mina_curves::pasta::{Fp, Fq, Pallas};
use o1_utils::FieldHelpers;
use serde::Deserialize;

use super::type_aliases::{WrapECPoint, WrapScalar};

type DecimalSigned = String;
type HexPointCoordinates = [String; 2];
type HexScalar = String;

#[derive(Deserialize)]
pub struct StateProof {
    pub proof: Proof,
    pub statement: Statement,
}

#[derive(Deserialize)]
pub struct Proof {
    pub commitments: Commitments,
    pub evaluations: Evaluations,
    pub ft_eval1: HexScalar,
    pub bulletproof: Bulletproof,
}

#[derive(Deserialize)]
pub struct Bulletproof {
    pub challenge_polynomial_commitment: HexPointCoordinates,
    pub delta: HexPointCoordinates,
    pub lr: [[HexPointCoordinates; 2]; 15],
    pub z_1: HexScalar,
    pub z_2: HexScalar,
}

#[derive(Deserialize)]
pub struct Commitments {
    pub w_comm: [HexPointCoordinates; 15],
    pub z_comm: HexPointCoordinates,
    pub t_comm: Vec<HexPointCoordinates>,
}

#[derive(Deserialize)]
pub struct Evaluations {
    pub coefficients: [HexPointCoordinates; 15],
    pub complete_add_selector: HexPointCoordinates,
    pub emul_selector: HexPointCoordinates,
    pub endomul_scalar_selector: HexPointCoordinates,
    pub generic_selector: HexPointCoordinates,
    pub mul_selector: HexPointCoordinates,
    pub poseidon_selector: HexPointCoordinates,
    pub s: [HexPointCoordinates; 6],
    pub w: [HexPointCoordinates; 15],
    pub z: HexPointCoordinates,
}

#[derive(Deserialize)]
pub struct Statement {
    pub messages_for_next_step_proof: MessagesForNextStepProof,
}

#[derive(Deserialize)]
pub struct MessagesForNextStepProof {
    pub challenge_polynomial_commitments: [HexPointCoordinates; 2],
    pub old_bulletproof_challenges: [[BulletproofChallenge; 16]; 2],
}

#[derive(Deserialize)]
pub struct BulletproofChallenge {
    pub prechallenge: Prechallenge,
}

#[derive(Deserialize)]
pub struct Prechallenge {
    // OCaml doesn't support unsigned integers, these should
    // be two u64 limbs but are encoded with a sign.
    // We just need to do a cast to u64.
    pub inner: [DecimalSigned; 2],
}

#[derive(Deserialize)]
pub struct ProofState {
    pub deferred_values: DeferredValues,
    pub messages_for_next_wrap_proof: MessagesForNextWrapProof,
    pub sponge_digest_before_evaluations: [HexScalar; 4],
}

#[derive(Deserialize)]
pub struct DeferredValues {
    pub branch_data: BranchData,
    pub bulletproof_challenges: [BulletproofChallenge; 16],
    pub plonk: Plonk,
}

#[derive(Deserialize)]
pub struct BranchData {
    pub domain_log2: String,
    pub proofs_verified: [String; 1],
}

#[derive(Deserialize)]
pub struct Plonk {
    pub alpha: Prechallenge,
    pub beta: HexPointCoordinates,
    pub feature_flags: FeatureFlags,
    pub gamma: HexPointCoordinates,
    pub zeta: Prechallenge,
}

#[derive(Deserialize)]
pub struct FeatureFlags {
    pub foreign_field_add: bool,
    pub foreign_field_mul: bool,
    pub lookup: bool,
    pub range_check0: bool,
    pub range_check1: bool,
    pub rot: bool,
    pub runtime_tables: bool,
    pub xor: bool,
}

#[derive(Deserialize)]
pub struct MessagesForNextWrapProof {
    pub challenge_polynomial_commitment: HexPointCoordinates,
    pub old_bulletproof_challenges: [[BulletproofChallenge; 16]; 2],
}

impl TryFrom<HexPointCoordinates> for WrapECPoint {
    type Error = String;

    fn try_from(value: HexPointCoordinates) -> Result<Self, Self::Error> {
        let x = Fp::from_hex(&value[0]).map_err(|err| err.to_string())?;
        let y = Fp::from_hex(&value[1]).map_err(|err| err.to_string())?;
        Ok(WrapECPoint(Pallas::new(x, y, false)))
    }
}

impl TryFrom<HexScalar> for WrapScalar {
    type Error = String;

    fn try_from(value: HexScalar) -> Result<Self, Self::Error> {
        Fq::from_hex(&value)
            .map(WrapScalar)
            .map_err(|err| err.to_string())
    }
}

pub fn parse(mina_state_proof_vk_query_str: &str) -> Result<StateProof, String> {
    let mina_state_proof_vk_query: serde_json::Map<String, serde_json::Value> =
        serde_json::from_str(mina_state_proof_vk_query_str)
            .map_err(|err| format!("Could not parse mina state proof vk query: {err}"))?;
    let protocol_state_proof_json = mina_state_proof_vk_query
            .get("data")
            .and_then(|d| d.get("bestChain"))
            .and_then(|d| d.get(0))
            .and_then(|d| d.get("protocolStateProof"))
            .and_then(|d| d.get("json"))
            .ok_or("Could not parse protocol state proof: JSON structure upto protocolStateProof is unexpected")?;

    serde_json::from_value(protocol_state_proof_json.to_owned())
        .map_err(|err| format!("Could not parse mina state proof: {err}"))
}

#[cfg(test)]
mod tests {
    use super::parse;

    const MINA_STATE_PROOF_VK_QUERY: &str = include_str!(
        "../../../../../batcher/aligned/test_files/mina/mina_state_proof_vk_query.json"
    );

    #[test]
    fn parse_protocol_state_proof() {
        parse(MINA_STATE_PROOF_VK_QUERY).unwrap();
    }
}
