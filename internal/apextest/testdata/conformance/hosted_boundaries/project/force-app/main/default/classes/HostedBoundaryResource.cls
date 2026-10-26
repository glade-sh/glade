@RestResource(urlMapping='/synthetic-hosted-boundary/*')
global with sharing class HostedBoundaryResource {
 @HttpGet global static String observe() {
 Map<String,Object> observed=new Map<String,Object>();
 observed.put('analytics_enum',HostedBoundaryProbe.observe('analytics_enum'));
 observed.put('analytics_id',HostedBoundaryProbe.observe('analytics_id'));
 observed.put('analytics_uuid',HostedBoundaryProbe.observe('analytics_uuid'));
 observed.put('analytics_null',HostedBoundaryProbe.observe('analytics_null'));
 observed.put('analytics_string',HostedBoundaryProbe.observe('analytics_string'));
 observed.put('analytics_boolean',HostedBoundaryProbe.observe('analytics_boolean'));
 observed.put('analytics_object',HostedBoundaryProbe.observe('analytics_object'));
 observed.put('analytics_warn',HostedBoundaryProbe.observe('analytics_warn'));
 observed.put('analytics_warn_id',HostedBoundaryProbe.observe('analytics_warn_id'));
 observed.put('analytics_info_id',HostedBoundaryProbe.observe('analytics_info_id'));
 observed.put('analytics_number_id',HostedBoundaryProbe.observe('analytics_number_id'));
 observed.put('notice_missing_type',HostedBoundaryProbe.observe('notice_missing_type'));
 observed.put('notice_missing_target',HostedBoundaryProbe.observe('notice_missing_target'));
 observed.put('notice_missing_title',HostedBoundaryProbe.observe('notice_missing_title'));
 observed.put('notice_empty_title',HostedBoundaryProbe.observe('notice_empty_title'));
 observed.put('notice_bad_type',HostedBoundaryProbe.observe('notice_bad_type'));
 observed.put('notice_configured',HostedBoundaryProbe.observe('notice_configured'));
 observed.put('notice_page',HostedBoundaryProbe.observe('notice_page'));
 observed.put('notice_constructor',HostedBoundaryProbe.observe('notice_constructor'));
 return JSON.serialize(observed);
 }
}
