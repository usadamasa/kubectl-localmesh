package envoy

type Route struct {
	Host        string
	LocalPort   int
	ClusterName string
}

func BuildConfig(listenerPort int, routes []Route) map[string]any {
	var clusters []any
	var vhosts []any

	for _, r := range routes {
		clusters = append(clusters, map[string]any{
			"name":            r.ClusterName,
			"type":            "STATIC",
			"connect_timeout": "1s",
			"load_assignment": map[string]any{
				"cluster_name": r.ClusterName,
				"endpoints": []any{
					map[string]any{
						"lb_endpoints": []any{
							map[string]any{
								"endpoint": map[string]any{
									"address": map[string]any{
										"socket_address": map[string]any{
											"address":    "127.0.0.1",
											"port_value": r.LocalPort,
										},
									},
								},
							},
						},
					},
				},
			},
			"typed_extension_protocol_options": map[string]any{
				"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": map[string]any{
					"@type": "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
					"explicit_http_config": map[string]any{
						"http2_protocol_options": map[string]any{},
					},
				},
			},
		})

		vhosts = append(vhosts, map[string]any{
			"name":    r.ClusterName,
			"domains": []any{r.Host},
			"routes": []any{
				map[string]any{
					"match": map[string]any{"prefix": "/"},
					"route": map[string]any{
						"cluster": r.ClusterName,
						"timeout": "0s",
					},
				},
			},
		})
	}

	return map[string]any{
		"static_resources": map[string]any{
			"listeners": []any{
				map[string]any{
					"name": "listener_http",
					"address": map[string]any{
						"socket_address": map[string]any{
							"address":    "0.0.0.0",
							"port_value": listenerPort,
						},
					},
					"filter_chains": []any{
						map[string]any{
							"filters": []any{
								map[string]any{
									"name": "envoy.filters.network.http_connection_manager",
									"typed_config": map[string]any{
										"@type":                  "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
										"stat_prefix":            "ingress_http",
										"codec_type":             "AUTO",
										"http2_protocol_options": map[string]any{},
										"route_config": map[string]any{
											"name":          "local_route",
											"virtual_hosts": vhosts,
										},
										"http_filters": []any{
											map[string]any{
												"name": "envoy.filters.http.router",
												"typed_config": map[string]any{
													"@type": "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"clusters": clusters,
		},
	}
}
