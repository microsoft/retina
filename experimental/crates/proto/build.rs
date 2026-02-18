fn main() -> Result<(), Box<dyn std::error::Error>> {
    let proto_dir = "proto";

    tonic_prost_build::configure()
        .build_server(true)
        .build_client(true)
        // prost-build cannot box individual oneof variants, so suppress the
        // warning on the two generated enums that contain the large Flow type.
        .type_attribute(
            "observer.GetFlowsResponse.response_types",
            "#[allow(clippy::large_enum_variant)]",
        )
        .type_attribute(
            "observer.ExportEvent.response_types",
            "#[allow(clippy::large_enum_variant)]",
        )
        .compile_protos(
            &[
                "proto/flow/flow.proto",
                "proto/observer/observer.proto",
                "proto/relay/relay.proto",
                "proto/ipcache/ipcache.proto",
            ],
            &[proto_dir],
        )?;

    Ok(())
}
