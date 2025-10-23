package generator

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	json "github.com/goccy/go-json"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// jsonLog represents a single log entry with random values
type jsonLog struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Environment string    `json:"environment"`
	Location    string    `json:"location"`
	Message     string    `json:"message"`
}

// JSONLogGenerator generates JSON log data with configurable workers
type JSONLogGenerator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	meter   metric.Meter

	// Metrics
	jsonLogsGenerated metric.Int64Counter
	jsonActiveWorkers metric.Int64Gauge
	jsonWriteErrors   metric.Int64Counter
}

// severityLevels contains random log severity levels
var severityLevels = []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

// environments contains random environment values
var environments = []string{"production", "development", "staging"}

// locations contains random location values
var locations = []string{"us-east1", "us-west1"}

// NewJSONGenerator creates a new JSON log generator
func NewJSONGenerator(logger *zap.Logger, workers int, rate time.Duration) (*JSONLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	meter := otel.Meter("bindplane-loader-generator")

	// Initialize metrics
	jsonLogsGenerated, err := meter.Int64Counter(
		"bindplane-loader.generator.logs.generated",
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	jsonActiveWorkers, err := meter.Int64Gauge(
		"bindplane-loader.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	jsonWriteErrors, err := meter.Int64Counter(
		"bindplane-loader.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &JSONLogGenerator{
		logger:            logger,
		workers:           workers,
		rate:              rate,
		stopCh:            make(chan struct{}),
		meter:             meter,
		jsonLogsGenerated: jsonLogsGenerated,
		jsonActiveWorkers: jsonActiveWorkers,
		jsonWriteErrors:   jsonWriteErrors,
	}, nil
}

// Start starts the JSON log generator and writes data using the
// provided generator writer.
func (g *JSONLogGenerator) Start(writer generatorWriter) error {
	g.logger.Info("Starting JSON log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate))

	// Record initial active workers count
	g.jsonActiveWorkers.Record(context.Background(), int64(g.workers),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
			),
		),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the JSON log generator.
// This function expects to be called exactly once.
func (g *JSONLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping JSON log generator")

	// Record zero active workers
	g.jsonActiveWorkers.Record(ctx, 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
			),
		),
	)

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("All workers stopped gracefully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

// worker runs a single worker goroutine
func (g *JSONLogGenerator) worker(workerID int, writer generatorWriter) {
	defer g.wg.Done()

	g.logger.Debug("Starting worker", zap.Int("worker_id", workerID))

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0 // Never stop retrying

	backoffTicker := backoff.NewTicker(backoffConfig)
	defer backoffTicker.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker stopping", zap.Int("worker_id", workerID))
			return
		case <-backoffTicker.C:
			err := g.generateAndWriteLog(writer, workerID)
			if err != nil {
				g.logger.Error("Failed to write log",
					zap.Int("worker_id", workerID),
					zap.Error(err))
				continue
			}
			backoffConfig.Reset()
		}
	}
}

// generateAndWriteLog generates a random log and writes it
func (g *JSONLogGenerator) generateAndWriteLog(writer generatorWriter, workerID int) error {
	data, err := generateRandomLog()
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("generate random log: %w", err)
	}

	// Record logs generated counter
	g.jsonLogsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
			),
		),
	)

	// Write the data with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writer.Write(ctx, data); err != nil {
		// Classify error type
		errorType := "unknown"
		if ctx.Err() == context.DeadlineExceeded {
			errorType = "timeout"
		}
		g.recordWriteError(errorType, err)
		return err
	}

	return nil
}

// generateRandomLog creates a random log entry
func generateRandomLog() ([]byte, error) {
	// Use fast random generator with gosec nosec comment
	messageIndex := rand.Intn(len(logMessages))  // #nosec G404
	levelIndex := rand.Intn(len(severityLevels)) // #nosec G404
	envIndex := rand.Intn(len(environments))     // #nosec G404
	locationIndex := rand.Intn(len(locations))   // #nosec G404

	j := jsonLog{
		Timestamp:   time.Now(),
		Level:       severityLevels[levelIndex],
		Environment: environments[envIndex],
		Location:    locations[locationIndex],
		Message:     logMessages[messageIndex],
	}

	return json.Marshal(j)
}

// logMessages contains 100 unique log messages of approximately 500 bytes each
var logMessages = []string{
	"User authentication failed for user_id=12345, ip_address=192.168.1.100, reason=invalid_password, attempt_count=3, timestamp=2024-01-15T10:30:45Z, session_id=abc123def456, user_agent=Mozilla/5.0, location=us-east-1, service=auth-service",
	"Database connection pool exhausted, active_connections=100, max_connections=100, waiting_requests=25, pool_name=primary_db, host=db-cluster-1.internal, port=5432, database=production_db, query_timeout=30s, retry_count=3",
	"Cache miss detected for key=user_profile_67890, cache_type=redis, ttl=3600s, hit_rate=0.85, memory_usage=2.1GB, max_memory=4GB, eviction_policy=LRU, cluster_nodes=3, replication_factor=2, compression_enabled=true",
	"API rate limit exceeded for client_id=api_client_123, endpoint=/api/v1/users, requests_per_minute=1000, limit=500, remaining_requests=0, reset_time=2024-01-15T10:31:00Z, client_ip=10.0.1.50, user_agent=MyApp/1.0",
	"File upload completed successfully, file_id=file_789012, file_size=15.2MB, file_type=image/jpeg, upload_duration=2.3s, storage_provider=S3, bucket=user-uploads, checksum=sha256:abc123, compression_ratio=0.75",
	"Background job processing started, job_id=job_456789, job_type=email_notification, queue_name=high_priority, estimated_duration=5m, retry_count=0, max_retries=3, timeout=30m, worker_id=worker_001, priority=high",
	"Memory usage threshold exceeded, current_usage=3.8GB, threshold=3.5GB, heap_size=4GB, gc_frequency=2.1s, goroutine_count=1250, cpu_usage=45%, disk_io=120MB/s, network_io=50MB/s, uptime=7d12h",
	"External API call failed, service=payment_gateway, endpoint=https://api.stripe.com/v1/charges, status_code=500, response_time=2.1s, retry_count=2, max_retries=3, error_message=Internal server error, request_id=req_123456",
	"Database query performance degraded, query_hash=abc123def456, execution_time=850ms, normal_time=50ms, rows_examined=1000000, rows_returned=100, index_usage=false, table_name=user_events, database=analytics_db",
	"Kubernetes pod restarted, pod_name=api-server-7d8f9g, namespace=production, restart_count=3, reason=OOMKilled, memory_limit=2GB, cpu_limit=1000m, node_name=worker-node-2, cluster=prod-cluster, image_tag=v1.2.3",
	"Message queue consumer lag detected, queue_name=order_processing, consumer_group=order_workers, lag_count=5000, processing_rate=100/s, consumer_count=5, partition_count=10, retention_policy=7d, compression=gzip",
	"SSL certificate expiration warning, domain=api.example.com, expiration_date=2024-02-15T00:00:00Z, days_remaining=30, certificate_authority=Let's Encrypt, key_size=2048, algorithm=RSA, auto_renewal=true",
	"Load balancer health check failed, backend_server=10.0.1.100:8080, health_check_path=/health, response_time=5.2s, timeout=3s, consecutive_failures=3, max_failures=5, status_code=503, error_message=Service unavailable",
	"Distributed tracing span completed, trace_id=1234567890abcdef, span_id=abcdef1234567890, operation_name=user_login, duration=150ms, tags=map[user_id:12345 service:auth], parent_span_id=9876543210fedcba",
	"Configuration reload completed, config_file=/etc/app/config.yaml, reload_duration=500ms, changes_detected=true, affected_services=[api,worker,cache], validation_errors=0, backup_created=true, rollback_available=true",
	"Security scan completed, scan_type=vulnerability_assessment, target=web_application, vulnerabilities_found=2, critical_count=0, high_count=1, medium_count=1, scan_duration=15m, scanner_version=2.1.0",
	"Backup operation started, backup_type=full_database, target_database=production_db, estimated_size=50GB, compression_enabled=true, encryption_enabled=true, retention_policy=30d, storage_location=S3://backups/",
	"Service discovery registration updated, service_name=user-service, service_version=v2.1.0, instance_id=user-svc-001, health_check_url=http://localhost:8080/health, metadata=map[region:us-east-1 environment:production]",
	"Circuit breaker opened, service_name=payment-service, failure_threshold=5, failure_count=5, timeout=30s, half_open_timeout=60s, last_failure_time=2024-01-15T10:30:00Z, error_rate=0.8, request_count=100",
	"Metrics collection completed, metric_name=request_duration, collection_interval=60s, data_points=1000, p50=50ms, p95=200ms, p99=500ms, max=1.2s, min=10ms, average=75ms, standard_deviation=45ms",
	"Log aggregation pipeline processing, log_source=application_logs, log_count=10000, processing_time=2.5s, parsing_errors=5, filtering_rules_applied=10, output_destinations=[elasticsearch,datadog], compression_ratio=0.6",
	"Container orchestration event, event_type=pod_scheduled, pod_name=worker-pod-abc123, namespace=default, node_name=worker-node-3, cpu_request=500m, memory_request=1Gi, storage_request=10Gi, priority_class=high",
	"Network traffic analysis completed, analysis_period=1h, total_packets=1000000, bytes_transferred=500MB, protocol_distribution=map[TCP:80% UDP:15% HTTP:5%], top_sources=[10.0.1.0/24,10.0.2.0/24], anomalies_detected=2",
	"Data pipeline processing started, pipeline_id=etl_pipeline_001, source_system=database, target_system=data_warehouse, estimated_records=1000000, batch_size=10000, parallel_workers=5, checkpoint_interval=5m",
	"Feature flag evaluation completed, flag_name=enable_new_checkout, user_id=12345, evaluation_result=true, rollout_percentage=50%, user_segment=beta_users, experiment_id=exp_001, variant=control",
	"Resource quota exceeded, resource_type=memory, namespace=production, current_usage=8GB, limit=10GB, warning_threshold=8GB, critical_threshold=9GB, scaling_recommendation=horizontal, estimated_cost_increase=20%",
	"Audit log entry created, event_type=user_permission_change, user_id=admin_001, target_user=user_12345, permission_added=read_write, resource=user_data, timestamp=2024-01-15T10:30:45Z, ip_address=192.168.1.1",
	"Performance benchmark completed, benchmark_name=cpu_intensive_task, execution_time=2.5s, cpu_usage=95%, memory_usage=512MB, iterations=1000, throughput=400 ops/s, latency_p50=2ms, latency_p99=10ms",
	"Deployment pipeline stage completed, pipeline_id=deploy_001, stage_name=integration_tests, duration=15m, test_count=500, passed_tests=495, failed_tests=5, coverage_percentage=85%, artifacts_generated=true",
	"Monitoring alert triggered, alert_name=high_error_rate, severity=warning, current_value=5.2%, threshold=5%, duration=10m, affected_services=[api,worker], notification_channels=[email,slack], escalation_policy=immediate",
	"Cache warming operation completed, cache_type=application_cache, keys_warmed=10000, warming_duration=30s, hit_rate_improvement=15%, memory_usage_increase=200MB, ttl_set=3600s, compression_enabled=true",
	"Database migration executed, migration_id=20240115_add_user_preferences, database=production_db, execution_time=2m, rows_affected=50000, rollback_available=true, backup_created=true, downtime=0s",
	"API versioning deprecation notice, deprecated_version=v1, current_version=v2, sunset_date=2024-06-15T00:00:00Z, migration_guide_url=https://docs.example.com/migration, affected_endpoints=[/api/v1/users,/api/v1/orders]",
	"Load testing results summary, test_name=peak_traffic_simulation, duration=1h, concurrent_users=1000, requests_per_second=500, average_response_time=200ms, error_rate=0.1%, throughput=450 RPS, resource_utilization=75%",
	"Security incident investigation, incident_id=SEC-2024-001, severity=medium, affected_systems=[web_app,database], initial_detection=2024-01-15T09:00:00Z, investigation_status=ongoing, evidence_collected=true",
	"Compliance audit checkpoint, audit_type=SOC2, checkpoint_name=data_encryption, status=compliant, evidence_count=25, remediation_items=0, next_audit_date=2024-07-15T00:00:00Z, auditor=external_firm",
	"Disaster recovery test completed, test_type=full_system_failover, rto_target=4h, rpo_target=1h, actual_rto=3.5h, actual_rpo=45m, test_status=successful, data_integrity_verified=true, rollback_time=30m",
	"Cost optimization analysis completed, analysis_period=30d, current_monthly_cost=$50000, optimization_potential=$5000, recommendations=[rightsize_instances,enable_spot_instances,optimize_storage], estimated_savings=10%",
	"Third-party integration health check, service_name=stripe_payments, status=healthy, response_time=150ms, error_rate=0.01%, last_incident=2024-01-10T14:30:00Z, sla_compliance=99.9%, monitoring_enabled=true",
	"Data retention policy enforcement, policy_name=user_logs_retention, retention_period=90d, records_processed=100000, records_deleted=5000, storage_freed=2GB, compliance_status=compliant, next_enforcement=2024-02-15",
	"Performance optimization applied, optimization_type=database_query, query_hash=abc123def456, improvement_percentage=60%, execution_time_before=1s, execution_time_after=400ms, index_added=true, cache_enabled=true",
	"Service mesh traffic routing updated, service_name=user-service, routing_rule=traffic_split, version_a_weight=80%, version_b_weight=20%, canary_deployment=true, rollback_threshold=5%, monitoring_duration=24h",
	"Infrastructure as Code deployment, template_name=kubernetes_cluster, environment=staging, resources_created=15, deployment_time=10m, validation_status=passed, drift_detected=false, compliance_check=passed",
	"Machine learning model training completed, model_name=recommendation_engine, training_data_size=1TB, training_duration=4h, accuracy_score=0.92, precision=0.89, recall=0.91, f1_score=0.90, model_version=v2.1",
	"Blockchain transaction processed, transaction_hash=0x1234567890abcdef, block_number=18500000, gas_used=21000, gas_price=20 gwei, transaction_fee=0.00042 ETH, confirmation_time=2m, network=ethereum_mainnet",
	"Edge computing workload deployed, workload_name=image_processing, edge_location=us-west-2, latency_reduction=200ms, bandwidth_savings=50%, processing_time=100ms, data_transfer_reduction=80%, cost_savings=30%",
	"Quantum computing simulation completed, simulation_type=molecular_dynamics, qubits_used=50, simulation_time=1h, accuracy_level=high, classical_equivalent_time=1000h, quantum_advantage=1000x, results_validated=true",
	"Augmented reality session analytics, session_id=ar_session_789, user_id=user_456, session_duration=15m, interactions_count=25, objects_recognized=10, tracking_accuracy=95%, battery_usage=15%, heat_generation=low",
	"Virtual reality performance metrics, vr_session_id=vr_123456, frame_rate=90fps, latency=20ms, resolution=4K, refresh_rate=120Hz, motion_sickness_incidents=0, user_comfort_score=9/10, hardware_utilization=85%",
	"IoT device telemetry processing, device_id=iot_sensor_001, data_points=1000, processing_latency=50ms, data_quality_score=98%, anomaly_detection_enabled=true, anomalies_found=2, alert_threshold=5%",
	"5G network optimization, cell_id=cell_001, signal_strength=-70dBm, throughput=1Gbps, latency=1ms, connection_count=1000, handover_success_rate=99.5%, interference_level=low, optimization_applied=true",
	"Autonomous vehicle data processing, vehicle_id=av_001, route_distance=50km, processing_time=2h, sensor_data_points=1000000, decision_points=500, safety_score=99.9%, fuel_efficiency=15% improvement",
	"Smart city infrastructure monitoring, city_id=smart_city_001, sensors_count=10000, data_collection_rate=1Hz, anomaly_detection_enabled=true, alerts_generated=5, response_time=2m, citizen_satisfaction=95%",
	"Digital twin synchronization, twin_id=manufacturing_line_001, sync_frequency=1s, data_points_synced=50000, accuracy_score=99.8%, latency=10ms, model_complexity=high, real_time_updates=true, prediction_accuracy=92%",
	"Robotic process automation execution, rpa_bot_id=invoice_processor_001, tasks_completed=100, execution_time=30m, success_rate=98%, error_count=2, human_intervention_required=false, cost_savings=$500",
	"Natural language processing pipeline, nlp_task=sentiment_analysis, text_samples=10000, processing_time=5m, accuracy_score=94%, language_support=5, model_version=v3.2, confidence_threshold=0.8",
	"Computer vision object detection, image_count=5000, objects_detected=25000, detection_accuracy=96%, processing_time=10m, model_type=YOLOv8, confidence_threshold=0.7, false_positive_rate=2%",
	"Speech recognition processing, audio_duration=2h, words_processed=20000, recognition_accuracy=97%, language_model=whisper_large, processing_time=30m, noise_reduction_applied=true, speaker_diarization=true",
	"Recommendation system update, system_name=content_recommender, user_count=100000, items_catalog=1000000, recommendation_accuracy=89%, click_through_rate=12%, conversion_rate=3.5%, model_retraining_frequency=daily",
	"Fraud detection analysis, transaction_count=1000000, fraud_cases_detected=100, false_positive_rate=0.1%, detection_time=50ms, model_accuracy=99.5%, risk_score_threshold=0.7, investigation_queue_size=25",
	"Supply chain optimization, optimization_type=route_planning, routes_optimized=1000, cost_reduction=15%, delivery_time_improvement=20%, fuel_savings=10%, carbon_footprint_reduction=12%, customer_satisfaction=95%",
	"Energy consumption monitoring, facility_id=facility_001, power_consumption=500kW, efficiency_score=85%, peak_demand=600kW, off_peak_usage=400kW, renewable_percentage=30%, cost_optimization_applied=true",
	"Water quality monitoring, sensor_network_id=water_network_001, ph_level=7.2, temperature=20°C, turbidity=0.5 NTU, chlorine_residual=0.8mg/L, monitoring_frequency=continuous, alert_thresholds=configured",
	"Air quality assessment, monitoring_station_id=air_station_001, pm2.5=15μg/m³, pm10=25μg/m³, ozone=80ppb, no2=30ppb, co=2ppm, air_quality_index=good, health_recommendations=none",
	"Waste management optimization, collection_route_id=route_001, collection_efficiency=92%, fuel_consumption=50L, route_distance=100km, collection_time=4h, recycling_rate=65%, waste_reduction=20%",
	"Traffic flow analysis, intersection_id=int_001, vehicle_count=5000, average_wait_time=30s, signal_timing_optimized=true, congestion_reduction=25%, fuel_savings=15%, emission_reduction=20%",
	"Parking management system, parking_lot_id=lot_001, occupancy_rate=75%, revenue_optimization=10%, payment_methods=[mobile,cash,card], reservation_system=true, dynamic_pricing_enabled=true",
	"Public transportation optimization, route_id=bus_route_001, passenger_count=1000, on_time_performance=95%, fuel_efficiency=8mpg, passenger_satisfaction=90%, route_optimization_applied=true",
	"Emergency response coordination, incident_id=emergency_001, response_time=5m, resources_deployed=10, coordination_efficiency=95%, communication_channels=5, status_updates=real_time, escalation_protocol=active",
	"Healthcare data analytics, patient_count=50000, data_points=1000000, privacy_compliance=HIPAA, analysis_type=predictive_modeling, accuracy_score=91%, model_explainability=high, bias_detection=enabled",
	"Educational platform analytics, student_count=10000, course_completion_rate=85%, engagement_score=88%, learning_outcomes=measured, adaptive_learning=enabled, personalized_content=95%, progress_tracking=continuous",
	"Financial risk assessment, portfolio_value=$100M, risk_score=medium, volatility=15%, correlation_analysis=completed, stress_testing=passed, regulatory_compliance=confirmed, reporting_frequency=monthly",
	"Insurance claim processing, claim_count=1000, processing_time=2d, fraud_detection_score=0.3, approval_rate=85%, customer_satisfaction=92%, automation_level=80%, human_review_required=20%",
	"Real estate market analysis, property_count=5000, market_trends=analyzed, price_prediction_accuracy=89%, demand_forecasting=enabled, location_analysis=comprehensive, investment_recommendations=generated",
	"Retail inventory optimization, store_count=100, inventory_turnover=6x, stockout_rate=2%, overstock_rate=5%, demand_forecasting_accuracy=87%, replenishment_automation=90%, cost_savings=12%",
	"Manufacturing quality control, production_line_id=line_001, defect_rate=0.5%, quality_score=99.5%, inspection_automation=95%, predictive_maintenance=enabled, equipment_efficiency=92%, waste_reduction=15%",
	"Agriculture precision farming, field_id=field_001, crop_yield_prediction=95% accuracy, soil_analysis=completed, irrigation_optimization=applied, pest_detection=enabled, harvest_timing=optimized",
	"Logistics route optimization, delivery_count=10000, route_efficiency=88%, fuel_savings=18%, delivery_time_reduction=25%, customer_satisfaction=94%, real_time_tracking=enabled, dynamic_routing=active",
	"Customer service analytics, ticket_count=5000, resolution_time=2h, customer_satisfaction=90%, first_call_resolution=75%, agent_productivity=increased 20%, chatbot_usage=60%, escalation_rate=15%",
	"Marketing campaign optimization, campaign_id=campaign_001, reach=1000000, engagement_rate=5%, conversion_rate=2.5%, roi=300%, audience_targeting=precise, ad_placement=optimized, budget_allocation=efficient",
	"Social media sentiment analysis, posts_analyzed=100000, sentiment_score=positive 70%, trending_topics=identified, influencer_impact=measured, engagement_prediction=85% accuracy, content_recommendation=personalized",
	"E-commerce recommendation engine, product_catalog=1000000, recommendation_accuracy=89%, click_through_rate=12%, conversion_rate=4%, personalization_level=high, cross_sell_success=25%, up_sell_success=15%",
	"Content management optimization, content_count=50000, search_accuracy=92%, content_discovery=improved, user_engagement=increased 30%, content_lifecycle=automated, version_control=comprehensive",
	"User experience analytics, session_count=100000, bounce_rate=35%, session_duration=3m, conversion_funnel=analyzed, user_journey=mapped, pain_points=identified, optimization_recommendations=generated",
	"Mobile app performance monitoring, app_version=v2.1.0, crash_rate=0.1%, load_time=2s, user_retention=85%, feature_adoption=measured, a/b_testing=active, performance_optimization=continuous",
	"Web application security scanning, vulnerability_count=2, security_score=95%, penetration_testing=completed, ssl_grade=A+, security_headers=configured, access_control=implemented, data_encryption=enabled",
	"API gateway traffic analysis, request_count=1000000, response_time=100ms, error_rate=0.1%, rate_limiting=active, authentication_success=99.5%, caching_hit_rate=80%, load_balancing=optimal",
	"Microservices communication monitoring, service_count=50, inter_service_calls=10000, latency_p95=50ms, circuit_breaker_status=healthy, service_discovery=active, load_balancing=round_robin",
	"Container orchestration metrics, pod_count=200, cpu_utilization=70%, memory_usage=80%, storage_utilization=60%, network_throughput=1Gbps, scaling_events=5, health_checks=passing",
	"Serverless function execution, function_count=100, invocations=50000, cold_starts=5%, execution_time=200ms, memory_usage=128MB, error_rate=0.05%, cost_optimization=applied, monitoring=comprehensive",
	"Edge computing workload distribution, edge_nodes=50, workload_distribution=balanced, latency_reduction=100ms, bandwidth_savings=40%, processing_efficiency=95%, failover_capability=enabled",
	"Multi-cloud resource management, cloud_providers=3, resource_count=1000, cost_optimization=15%, performance_monitoring=active, disaster_recovery=configured, compliance=maintained, security=unified",
	"Hybrid cloud integration, on_premise_resources=500, cloud_resources=1000, data_synchronization=real_time, security_policy=unified, compliance_governance=centralized, cost_transparency=complete",
	"Infrastructure monitoring dashboard, metrics_collected=1000, alert_rules=100, dashboard_widgets=50, data_retention=90d, visualization_types=10, real_time_updates=enabled, historical_analysis=available",
	"DevOps pipeline automation, pipeline_stages=10, automation_level=95%, deployment_frequency=daily, lead_time=2h, mean_time_to_recovery=30m, change_failure_rate=5%, deployment_success_rate=98%",
	"Continuous integration metrics, build_count=1000, build_success_rate=95%, test_coverage=85%, code_quality_score=92%, security_scanning=automated, dependency_updates=monitored, artifact_management=versioned",
	"Continuous deployment analytics, deployment_count=500, rollback_rate=2%, feature_flag_usage=80%, canary_deployment_success=95%, blue_green_deployment=active, database_migration=automated",
	"Configuration management tracking, configuration_items=1000, change_frequency=daily, compliance_status=maintained, drift_detection=active, version_control=git, approval_workflow=automated",
	"Secrets management audit, secret_count=500, rotation_frequency=90d, access_logging=enabled, encryption_at_rest=true, encryption_in_transit=true, compliance_audit=passed, breach_detection=active",
	"Identity and access management, user_count=10000, role_count=100, permission_count=1000, access_reviews=quarterly, privileged_access=monitored, multi_factor_authentication=95%, single_sign_on=enabled",
	"Security information and event management, event_count=1000000, correlation_rules=500, false_positive_rate=5%, incident_response_time=15m, threat_intelligence=integrated, compliance_reporting=automated",
	"Vulnerability management program, vulnerability_count=100, critical_count=5, high_count=20, medium_count=50, low_count=25, remediation_time=30d, patch_management=automated, risk_assessment=continuous",
	"Penetration testing results, test_scope=full_application, vulnerabilities_found=10, critical_findings=2, remediation_timeline=60d, retest_scheduled=true, compliance_requirements=met, security_posture=improved",
	"Security awareness training, employee_count=1000, training_completion=95%, phishing_simulation_success=85%, security_incident_reduction=40%, knowledge_assessment_score=90%, training_frequency=quarterly",
	"Business continuity planning, recovery_time_objective=4h, recovery_point_objective=1h, backup_frequency=hourly, disaster_recovery_testing=quarterly, failover_capability=verified, communication_plan=active",
	"Risk management assessment, risk_categories=10, risk_count=100, mitigation_strategies=implemented, residual_risk=low, risk_appetite=defined, risk_monitoring=continuous, board_reporting=monthly",
	"Compliance monitoring dashboard, regulatory_frameworks=5, compliance_score=95%, audit_findings=2, remediation_status=in_progress, policy_updates=monthly, training_completion=98%, violation_tracking=active",
	"Data governance framework, data_classification=5_tiers, data_lineage=mapped, privacy_impact_assessment=completed, data_retention_policy=enforced, access_controls=implemented, audit_trail=comprehensive",
	"Privacy compliance management, gdpr_compliance=verified, ccpa_compliance=verified, data_subject_requests=processed, consent_management=active, privacy_by_design=implemented, data_protection_officer=assigned",
	"Information security policy enforcement, policy_count=20, policy_awareness=95%, policy_violations=5, enforcement_actions=taken, policy_reviews=annual, updates_communicated=immediately, training_mandatory=true",
	"Cybersecurity incident response, incident_count=10, response_time=15m, containment_time=30m, eradication_time=2h, recovery_time=4h, lessons_learned=documented, process_improvement=implemented",
	"Threat hunting operations, hunting_techniques=10, threat_indicators=1000, false_positive_rate=10%, detection_capability=enhanced, threat_intelligence=integrated, hunting_automation=50%, analyst_productivity=increased",
	"Digital forensics investigation, case_count=5, evidence_collection=comprehensive, chain_of_custody=maintained, analysis_tools=validated, reporting_standards=met, expert_testimony=prepared, legal_compliance=verified",
	"Security architecture review, architecture_components=50, security_controls=200, threat_modeling=completed, security_requirements=defined, design_reviews=conducted, implementation_guidance=provided, validation_testing=planned",
	"Application security testing, application_count=100, static_analysis=automated, dynamic_analysis=scheduled, interactive_analysis=manual, code_review=mandatory, security_training=developer_focused, vulnerability_tracking=integrated",
	"Network security monitoring, network_segments=20, monitoring_coverage=100%, traffic_analysis=continuous, anomaly_detection=enabled, intrusion_prevention=active, firewall_rules=optimized, network_segmentation=implemented",
	"Endpoint detection and response, endpoint_count=5000, detection_rules=500, response_automation=80%, threat_intelligence=integrated, behavioral_analysis=enabled, incident_correlation=active, forensic_capability=built_in",
	"Cloud security posture management, cloud_accounts=10, resource_count=10000, misconfiguration_detection=automated, compliance_monitoring=continuous, security_baseline=defined, remediation_automation=60%, cost_optimization=security_focused",
	"Data loss prevention, data_classification=automatic, policy_count=50, violation_count=100, false_positive_rate=15%, encryption_enforcement=active, access_monitoring=continuous, incident_response=automated",
	"Email security gateway, email_volume=100000, spam_detection=99%, malware_blocking=100%, phishing_prevention=95%, data_leak_prevention=active, encryption_enforcement=mandatory, compliance_filtering=enabled",
	"Web application firewall, request_count=1000000, attack_blocking=99.9%, false_positive_rate=0.1%, rule_updates=automatic, threat_intelligence=integrated, performance_impact=minimal, ssl_inspection=enabled",
	"Database security monitoring, database_count=50, query_monitoring=continuous, access_logging=enabled, privilege_escalation=detected, data_encryption=at_rest_and_in_transit, backup_encryption=enabled, audit_trail=comprehensive",
	"Identity governance and administration, identity_count=10000, lifecycle_management=automated, access_certification=quarterly, segregation_of_duties=enforced, privileged_access=monitored, compliance_reporting=automated",
	"Security orchestration and automation, playbook_count=50, automation_level=80%, response_time=reduced_75%, analyst_productivity=increased_50%, false_positive_reduction=60%, integration_count=100, workflow_efficiency=optimized",
}

// recordWriteError records metrics for write errors
func (g *JSONLogGenerator) recordWriteError(errorType string, err error) {
	ctx := context.Background()

	g.jsonWriteErrors.Add(ctx, 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
				attribute.String("error_type", errorType),
			),
		),
	)
}
