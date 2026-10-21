@IsTest private class SurfaceSliceTest {
 @IsTest static void callbackFields() {
  Auth.AuthProviderTokenResponse value=new Auth.AuthProviderTokenResponse('synthetic-provider','synthetic-token','synthetic-refresh','synthetic-state');
  System.assert(false,'OBSERVED|callbackFields|'+JSON.serialize(new Map<String,Object>{'provider'=>value.provider,'token'=>value.oauthToken,'refresh'=>value.oauthSecretOrRefreshToken,'state'=>value.state,'idToken'=>value.idToken}));
 }
 @IsTest static void refreshFields() {
  Auth.OAuthRefreshResult value=new Auth.OAuthRefreshResult('synthetic-token','synthetic-refresh');
  System.assert(false,'OBSERVED|refreshFields|'+JSON.serialize(new Map<String,Object>{'accessToken'=>value.accessToken,'refreshToken'=>value.refreshToken,'error'=>value.error}));
 }
 @IsTest static void refreshErrorFields() {
  Auth.OAuthRefreshResult value=new Auth.OAuthRefreshResult('synthetic-token','synthetic-refresh','synthetic-error');
  System.assert(false,'OBSERVED|refreshErrorFields|'+JSON.serialize(new Map<String,Object>{'accessToken'=>value.accessToken,'refreshToken'=>value.refreshToken,'error'=>value.error}));
 }
 @IsTest static void nullFields() {
  Auth.AuthProviderTokenResponse value=new Auth.AuthProviderTokenResponse(null,null,null,null);
  Auth.OAuthRefreshResult result=new Auth.OAuthRefreshResult(null,null,null);
  System.assert(false,'OBSERVED|nullFields|'+JSON.serialize(new Map<String,Object>{'provider'=>value.provider,'token'=>value.oauthToken,'refresh'=>value.oauthSecretOrRefreshToken,'state'=>value.state,'accessToken'=>result.accessToken,'refreshToken'=>result.refreshToken,'error'=>result.error}));
 }
}
